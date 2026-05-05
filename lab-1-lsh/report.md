# lab-1-lsh

## Структура
- `lsh/` — индекс MinHash + LSH для текстов
- `tests/func_test.go` — функциональные тесты
- `tests/perf_test.go` — бенчмарки `Build`, `Add`, `FindDuplicates`, `FullScanDuplicates`

## Что профилировать
```bash
go test ./tests -run '^$' -bench BenchmarkLSHFindDuplicates -benchmem -cpuprofile cpu.out -memprofile mem.out
```

## На что смотреть в профиле
- построение шинглов
- расчёт minhash signature
- сортировка и merge-подобный Jaccard
- длинные candidate slices в band buckets

## Основные гипотезы улучшений
1. Для query-path переиспользовать scratch buffer под signature и shingles.
2. Для band buckets рассмотреть packed storage с периодической компактацией.
3. Если преподаватель захочет дальше ужимать аллокации — можно токенизировать без unicode fallback под ASCII-only corpora.
