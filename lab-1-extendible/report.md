# lab-1-extendible

## Структура
- `extendible/` — реализация extendible hashing на файловой системе
- `tests/func_test.go` — функциональные тесты
- `tests/perf_test.go` — бенчмарки по одной операции

## Что профилировать
```bash
go test ./tests -run '^$' -bench BenchmarkExtendibleInsert -benchmem -cpuprofile cpu.out -memprofile mem.out
```

## На что смотреть в профиле
- split/merge бакетов
- сериализация bucket/meta
- msync в `BenchmarkExtendibleSync`
- рост directory при неудачном распределении ключей

## Основные гипотезы улучшений
1. Перейти с линейного поиска внутри маленького бакета на компактный open-addressing bucket фиксированного размера.
2. Добавить page-granularity flush, если преподаватель захочет совсем жёсткий I/O разбор.
3. Для hot-path insert/update можно разделить dirty-meta и dirty-bucket flush по группам файлов.
