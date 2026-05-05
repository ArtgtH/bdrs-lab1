# lab-1-perfect

## Структура
- `perfect/` — immutable perfect hash без операций вставки после построения
- `tests/func_test.go` — функциональные тесты
- `tests/perf_test.go` — бенчмарки `Build` и `Get`

## Что профилировать
```bash
go test ./tests -run '^$' -bench BenchmarkPerfectBuild -benchmem -cpuprofile cpu.out -memprofile mem.out
```

## На что смотреть в профиле
- подбор seed первого уровня
- построение вторичных бакетов
- суммарный объём `m^2`-слотов во вторичном уровне

## Основные гипотезы улучшений
1. Выбирать seed первого уровня эвристикой по нескольким лучшим кандидатам с сортировкой bucket size descending.
2. Для very large buckets добавить переключение на альтернативную secondary scheme.
3. Уменьшить стоимость сравнения строк через хранение дополнительного 64-bit fingerprint в слоте.
