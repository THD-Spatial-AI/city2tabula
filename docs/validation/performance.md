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
SELECT COUNT(id) FROM city2tabula.lod2_surface_raw;
```

---

## Results

| Dataset | LoD | Buildings | Surface count | Wall time | Throughput | Peak PG RSS | PG idle baseline | PG increment |
| ------- | --- | --------- | ------------- | --------- | ---------- | ----------- | ---------------- | ------------ |
| Freiburg (DE) | 2 | 130,191 | 2,297,558 | 10m 28s (628 s) | 3,657 surf/s | 7.5 GB | 1.6 GB | 5.9 GB |
| Vienna (AT) | 2.1 | 130,000 | 2,753,166 | 21m 33s (1,293 s) | 2,129 surf/s | 8.6 GB | 2.3 GB | 6.3 GB |
| Deggendorf (DE) | 2 | 125,991 | 1,768,124 | 8m 11s (492 s) | 3,596 surf/s | 7.4 GB | 1.6 GB | 5.8 GB |

---
