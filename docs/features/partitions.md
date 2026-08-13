# Partitions

[← Features and UI behaviour](../features.md)

## What a partition is here

A partitioned table is one table whose rows are stored in named **pieces**. That is a fact about the
table's *shape* rather than a second kind of object, which is why a partition is drawn under the
table beside [Fields, Indexes and Foreign Keys](schema-tree.md#browsing-a-schema) and never in a
folder of the namespace — a reader does not look for `sales_2026` among the tables; they look at
`sales` and ask how it is cut.

**Three engines name one.** MySQL, PostgreSQL and Oracle each let a piece be addressed by name;
SQL Server's are *numbered* pieces of a partition scheme rather than named objects, and SQLite has
none at all. Those two draw no `Partitions` folder — `EngineInfo.hasPartitions` is what says so, and
[no component compares against an engine's name](../CLAUDE.md#rules-that-must-not-be-broken).

## What the folder shows

One row per piece, in the order the catalogue reports them, each badged with **what it holds**:

| Engine | Read from | What a badge says |
|---|---|---|
| MySQL | `information_schema.partitions` | `VALUES LESS THAN (2028)`, `VALUES IN (1, 3)` — put back together from `partition_description`, which keeps the value without the words |
| PostgreSQL | `pg_inherits` and `pg_get_expr(relpartbound)` | `FOR VALUES FROM ('2027-01-01') TO ('2028-01-01')` — the server's own words, unchanged |
| Oracle | `all_tab_partitions` joined to `all_part_tables` | `VALUES LESS THAN (2028)`, from the `high_value` the dictionary keeps |

**A table that is not partitioned has an empty folder**, exactly as a table with no index has an
empty `Indexes`. The two are not the same as an engine that cannot partition at all, and the
difference is the folder being drawn rather than a row inside it.

**What the table is cut *by* is repeated on every piece.** The method and the expression are facts of
the table and the catalogues answer for them that way; a list of pieces that did not say what it was
cut by would say very little.

**A PostgreSQL partition is a table** and is deliberately left out of the `Tables` folder. Listing it
in both places would put every piece of a partitioned table in the tree twice and make the `Tables`
count say something other than how many tables there are.

## Changing one

Three items, on the `Partitions` folder and on a piece:

| Item | What it does |
|---|---|
| `Add Partition…` | Asks for a **name**, then for **what it holds** |
| `Empty Partition` | Drops the rows and leaves the piece |
| `Delete Partition` | Drops the piece **and the rows in it** |

**What a piece holds is the reader's own clause, in the engine's own words.** The second prompt
starts from `VALUES LESS THAN (0)` and is edited, rather than being a form of boxes: that grammar is
different on every engine that has one *and* different again per partitioning method — range, list,
hash, interval, and each of those in its composite form — so a builder here would only ever cover
the ones somebody thought of, and the server is what checks the rest. It reaches the statement
verbatim, exactly as [a column's type does](schema-changes.md).

**Both removals are confirmed, and they say which is which.** `Empty` says the piece stays and the
rest of the table is untouched; `Delete` says the rows go with the piece *and* that rows which would
have landed there have nowhere to go — which is the consequence a reader is least likely to have in
mind. It is the same pair of words `Empty Table` and `Delete Table` are, one level down.

**One speller writes all three.** `sqlkit.DDL` spells every partition statement there is and what
differs between the engines is a **flag** — `sqlkit.PartitionStyle`, which has exactly three values:

- **`PartitionAsTable`** (PostgreSQL): a piece *is* a table, so it is declared with `CREATE TABLE …
  PARTITION OF …`, dropped with `DROP TABLE` and emptied with `TRUNCATE TABLE`. The one style whose
  statements never say the word `PARTITION` at all.
- **`PartitionInList`** (MySQL): an added piece sits inside a parenthesised list —
  `ADD PARTITION (PARTITION p …)`.
- **`PartitionAsClause`** (Oracle): the piece is named straight after the keyword.

Each of the three sits beside its spelling on `driver.Schema`, so what a
[migration](schema-changes.md) shows and what its Apply runs come from one function — [rule
13](../CLAUDE.md#rules-that-must-not-be-broken). An engine with no partition to address spells
**nothing**, and the run of that same change answers [`UNSUPPORTED`](../errors.md).

## What is not there

**A partition is not reorganised.** Splitting one, merging two, exchanging one with a table and
coalescing a hash partition are four more grammars again, every one of them different per engine,
and every one of them **moves rows** — which is a different promise from adding an empty piece or
dropping a named one. They are a query tab's business, where the server answers for itself.

**A table is not partitioned from here.** Cutting an existing table into pieces means declaring it
again and copying every row into the new one on three of these engines; what this offers is adding
to, emptying and dropping the pieces of a table that already is.

**A partition has no rows tab of its own**, even on the engine where it is a table. What a reader
wants from `sales_2026` is the rows of `sales` narrowed to it, which is
[a filter](filter-sort.md) on the table itself and is already there.

**A partition is not in a dump.** A dump is a table's shape and its rows, and a
[`CREATE TABLE`](transfer.md) it writes carries neither the partitioning clause nor the pieces —
replaying it makes an ordinary table holding the same rows.
