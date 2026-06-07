# Pipeline Performance

End-to-end runtime and memory benchmarks for the `city2tabula -extract-features` command across three municipality-scale datasets.

---

## Environment

| Component | Version / Spec |
| --------- | -------------- |
| CPU | Intel Core Ultra 9 185H (22 cores) |
| RAM | 62 GB |
| OS | Ubuntu 24.04 |
| PostgreSQL | 18.4 |
| PostGIS | 3.6 |

All datasets were fully loaded into 3DCityDB before timing started. No import or preprocessing time is included in the reported runtimes.

---

## How Numbers Were Collected

**Wall time and Go process RSS** — `/usr/bin/time -v`:

```bash
/usr/bin/time -v ./city2tabula -extract-features 2>&1 | tee run_<city>.log
```

**PostgreSQL RSS** — sampled every 500 ms while the pipeline ran:

```bash
(while true; do
    ps -o rss= -p $(pgrep -d, postgres) 2>/dev/null \
      | tr ',' '\n' | awk '{s+=$1} END {printf "%d\n", s/1024}';
    sleep 0.5;
done) > pg_mem_<city>.log &
```

Kill the monitor after the run:

```bash
kill $(jobs -p)
```

**Surface count** — after extraction completes:

```sql
SELECT COUNT(id) FROM city2tabula.lod2_child_feature_surface;
```

---

## Results

| Dataset | LoD | Buildings | Surface count | Wall time | Throughput | Peak PG RSS | PG idle baseline | PG increment |
| ------- | --- | --------- | ------------- | --------- | ---------- | ----------- | ---------------- | ------------ |
| Freiburg (DE) | 2 | 130,191 | 2,297,558 | 10m 28s (628 s) | 3,657 surf/s | 7.5 GB | 1.6 GB | 5.9 GB |
| Vienna (AT) | 2.1 | 130,000 | 2,753,166 | 21m 33s (1,293 s) | 2,129 surf/s | 8.6 GB | 2.3 GB | 6.3 GB |
| Deggendorf (DE) | 2 | 125,991 | 1,768,124 | 8m 11s (492 s) | 3,596 surf/s | 7.4 GB | 1.6 GB | 5.8 GB |

**Go process peak RSS** is ~20–25 MB across all runs — all heavy work executes inside PostgreSQL.

---

## Key Observations

### Throughput is determined by LoD, not dataset origin

The two LoD2 datasets (Freiburg, Deggendorf) process at ~3,600 surfaces/second despite coming from different data providers (Baden-Württemberg vs Bavaria). Vienna (LoD2.1) processes at ~2,100 surfaces/second — ~42% slower — due to higher vertex density and non-planar face count at finer detail levels.

### Memory scales sub-linearly with building count

The pipeline-attributable PostgreSQL memory increment is 5.8–6.3 GB across all three datasets, despite building counts ranging from ~126k to ~130k. The consistent increment confirms that memory usage is bounded by geometry complexity (surface count and vertex density) rather than raw building count.

### Invalid geometry in Vienna

One batch failed all retries on `04_calc_bld_feat.sql` with a GEOS `TopologyException` caused by invalid geometry in the Vienna source data. The pipeline completed with exit status 0, processing the remaining 129 of 130 batches. This is consistent with the known [wall area limitation for degenerate geometry](../known-issues/).

---

## Reproducing a Run

```bash
# 1. Start memory monitor
(while true; do
    ps -o rss= -p $(pgrep -d, postgres) 2>/dev/null \
      | tr ',' '\n' | awk '{s+=$1} END {printf "%d\n", s/1024}';
    sleep 0.5;
done) > pg_mem_<city>.log &

# 2. Run pipeline
/usr/bin/time -v ./city2tabula -extract-features 2>&1 | tee run_<city>.log

# 3. Stop monitor (same terminal)
kill $(jobs -p)

# 4. Read results
echo "Peak PostgreSQL RSS (MB): $(sort -n pg_mem_<city>.log | tail -1)"
echo "Baseline (idle, MB): $(head -1 pg_mem_<city>.log)"
grep -E "wall clock|Maximum resident" run_<city>.log

# 5. Surface count (with dataset DB loaded)
psql -U $DB_USER -d $DB_NAME -c \
    "SELECT COUNT(id) FROM city2tabula.lod2_child_feature_surface;"
```

See also: [Benchmarking Guide](../benchmark/benchmarking-guide.md) for full context on what each metric means.
