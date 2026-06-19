# Summary statistics

Use `atb summarise` to count and group genomes in the database or in a previous query result.

## Summarise the database

```bash
# Default summary of the full database
atb summarise

# Group by species (top 20)
atb summarise --by sylph_species --top 20

# Summarise a previous query result
atb query --genus Salmonella --hq-only --limit 100 \
  --columns sample_accession,sylph_species,hq_filter,dataset -o salmonella.tsv
atb summarise --from salmonella.tsv

# Pipe query to summarise
atb query --species "Escherichia coli" --hq-only --limit 200 \
  --columns sample_accession,sylph_species,dataset --format csv | \
  atb summarise --from -
```

## Related pages

- [Fetching & indexing data](fetch-and-index.md) — download the database before running `atb summarise`
- [Querying genomes](query.md) — filter genomes to feed into `atb summarise --from`
