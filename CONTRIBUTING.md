# Contributing

## Generated Documentation

Generated documentation examples are checked into the repository. Regenerate
them before committing changes that affect renderer output:

```shell
make gen
```

CI runs `make check-generated`, which regenerates the files and fails if
`README.md` or any file under `docs/examples` is stale.

## Checks

Run the same checks as CI before committing:

```shell
make test
make lint
```
