package app

import (
	"path/filepath"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

func importInput(mode string, mappings ...FieldMapping) ImportInput {
	return ImportInput{Table: "users", Mode: mode, Mappings: mappings}
}

func TestPlanCollectsOnlyTheMappedColumns(t *testing.T) {
	_, write, err := importInput("append",
		FieldMapping{Source: "a", Target: "id", Key: true},
		FieldMapping{Source: "", Target: "created_at"},
		FieldMapping{Source: "b", Target: "name"},
	).plan()
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// `created_at` is deliberately absent: a column the file has nothing for is left out of the
	// write so the server's own default applies, which is not the same as writing NULL into it.
	if len(write.Columns) != 2 || write.Columns[1] != "name" {
		t.Errorf("columns = %v, want [id name]", write.Columns)
	}
	if len(write.Keys) != 1 || write.Keys[0] != "id" {
		t.Errorf("keys = %v, want [id]", write.Keys)
	}
	if write.Batch != defaultImportBatch {
		t.Errorf("batch = %d, want %d", write.Batch, defaultImportBatch)
	}
}

// Every mode that finds a stored row before it touches it is refused without a key, here rather
// than at the server: the statement it would otherwise build reaches every row in the table.
func TestPlanRefusesAKeyedModeWithNoKey(t *testing.T) {
	for _, mode := range []string{"update", "upsert", "skip", "delete"} {
		_, _, err := importInput(mode, FieldMapping{Source: "a", Target: "id"}).plan()
		if err == nil {
			t.Errorf("%s was planned with no key", mode)
		}
	}

	if _, _, err := importInput("append", FieldMapping{Source: "a", Target: "id"}).plan(); err != nil {
		t.Errorf("append needs no key: %v", err)
	}
}

func TestPlanRefusesAMappingThatMatchesNothing(t *testing.T) {
	if _, _, err := importInput("append").plan(); err == nil {
		t.Error("plan accepted a mapping with no columns in it")
	}
	if _, _, err := importInput("plunder", FieldMapping{Source: "a", Target: "id"}).plan(); err == nil {
		t.Error("plan accepted a mode this app does not have")
	}
}

// Copy is an append with the table emptied first, which is why the two share a driver mode — and
// why only one of them is destructive.
func TestCopyIsAnAppendThatEmptiesFirst(t *testing.T) {
	mode, write, err := importInput("copy", FieldMapping{Source: "a", Target: "id"}).plan()
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if mode != domain.ImportCopy || write.Mode != driver.WriteAppend {
		t.Errorf("copy planned as %s / %s", mode, write.Mode)
	}
	if got := domain.ImportHazard(mode, true); got != domain.HazardDestructive {
		t.Errorf("copy hazard = %s, want destructive", got)
	}
}

// A row is reordered into the column order the plan collected, so a file whose fields are in another
// order than the table's still lands in the right cells.
func TestMappedReordersAndPadsARow(t *testing.T) {
	mappings := []FieldMapping{
		{Source: "b", Target: "id"},
		{Source: "a", Target: "name"},
		{Source: "missing", Target: "note"},
	}

	got := mapped(
		[]*string{text("Ada"), text("1")},
		index([]string{"a", "b"}),
		mappings,
	)

	if len(got) != 3 || *got[0] != "1" || *got[1] != "Ada" || got[2] != nil {
		t.Errorf("mapped = %v", got)
	}
}

func TestExportFileNameIsMadeSafeAndStaysInTheFolder(t *testing.T) {
	job := ExportJobInput{Format: "csv", Folder: "E:\\out"}

	path, err := job.pathFor("sales/2026")
	if err != nil {
		t.Fatalf("pathFor: %v", err)
	}
	if got := filepath.Base(path); got != "sales_2026.csv" {
		t.Errorf("file = %s, want sales_2026.csv", got)
	}

	// A separator is what a table name would climb out of the folder with, so it is what is
	// replaced — the file lands beside the others rather than wherever the name pointed.
	climbing, err := job.pathFor("../etc/passwd")
	if err != nil {
		t.Fatalf("pathFor: %v", err)
	}
	if filepath.Dir(climbing) != job.Folder {
		t.Errorf("wrote to %s, want a file inside %s", climbing, job.Folder)
	}
}
