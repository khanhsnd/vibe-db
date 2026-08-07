package domain

import "testing"

// A toolkit is a *pair*: a dump nothing can put back is worse than no dump at all, so an engine
// either has both programs or has neither.
func TestAToolkitIsAlwaysAPair(t *testing.T) {
	engines := []Engine{
		EngineMySQL, EnginePostgres, EngineSQLite,
		EngineSQLServer, EngineOracle, EngineMongo, EngineRedis,
	}

	for _, engine := range engines {
		kit, ok := engine.Toolkit()

		if ok != engine.HasToolkit() {
			t.Errorf("%s: HasToolkit disagrees with Toolkit", engine)
		}
		if !ok {
			if kit != (Toolkit{}) {
				t.Errorf("%s: has no toolkit but answered with one: %+v", engine, kit)
			}
			continue
		}

		if kit.Dump == "" || kit.Restore == "" {
			t.Errorf("%s: half a pair — dump %q, restore %q", engine, kit.Dump, kit.Restore)
		}
		if kit.Extension == "" || kit.Format == "" {
			t.Errorf("%s: a file with no name for what it is: %+v", engine, kit)
		}
	}
}

// A backup is a file of statements and a tool dump is the vendor's own bytes, so the two capability
// rules are independent — and MongoDB is the engine that proves it, having the second without the
// first. A test that let them drift into agreement would be a test of nothing.
func TestADocumentStoreHasToolsWithoutHavingBackups(t *testing.T) {
	if EngineMongo.Backups() {
		t.Error("a document store's commands are not a declaration and a row")
	}
	if !EngineMongo.HasToolkit() {
		t.Error("MongoDB ships mongodump, which is the whole reason this feature exists")
	}
	if EngineRedis.HasToolkit() {
		t.Error("Redis persists itself, where the server is, and no client program pulls that back")
	}
}
