## Задание

В работе реализованы и исследованы три алгоритма:

1. `extendible hashing` на файловой системе с операциями `Build`, `Get`, `Insert`, `Update`, `Delete`
2. `perfect hash` для фиксированного набора ключей с операциями `Build`, `Get`
3. `LSH` для поиска near-duplicate текстов с операциями `Build`, `Add`, `FindDuplicates`, `FullScanDuplicates`


## Реализация

### Perfect Hash

Реализация находится в `lab-1-perfect/perfect/`.

Использована двухуровневая схема:

1. на первом уровне подбирается `primarySeed`, минимизирующий сумму квадратов размеров бакетов;
2. для каждого бакета второго уровня строится собственная collision-free таблица;
3. для бакетов размера `m > 1` выделяется `m^2` слотов.

Ключевые свойства реализации:

1. структура immutable после построения;
2. lookup выполняется за одну primary и одну secondary хеш-функцию;

### LSH

Реализация находится в `lab-1-lsh/lsh/`.

Использована схема:

1. текст разбивается на токены;
2. из токенов строятся shingles фиксированного размера;
3. по shingles считается MinHash signature;
4. signature режется на bands;
5. документы попадают в band buckets;
6. `FindDuplicates` ищет кандидатов через LSH buckets, а `FullScanDuplicates` проверяет все документы полным проходом.

Ключевые свойства реализации:

1. индекс поддерживает `Build` и инкрементальный `Add`;
2. `FullScanDuplicates` нужен как baseline.

### Extendible Hashing

Реализация находится в `lab-1-extendible/extendible/`.

Использована схема:

1. directory с `global depth`;
2. buckets с `local depth`;
3. split при переполнении бакета;
4. merge и shrink directory при удалении;
5. хранение `meta.dat` и `bucket_*.dat` на диске.

Ключевые свойства реализации:

1. `Put` и `Delete` только помечают бакет как dirty через `markBucketDirty`, а не вызывают `Sync()` немедленно
2. реальный flush происходит в `Sync()` и `Close()`
3. `mmapCache.write()` только обновляет mmap-region и ставит `dirty = true`, а `msync` вызывается позже в `flushDirty()'
4. уже загруженные бакеты кэшируются в `ht.buckets`

## Методика измерений

Система: Intel® Core™ Ultra 7 255H × 16; 32GB RAM

Все benchmark-артефакты лежат в `artifacts/benchmarks/`.

1. `PerfectBuild`, `LSHBuild`, `LSHAdd`, `LSHFullScanDuplicates`: `10` прогонов
2. `PerfectGet`, `LSHFindDuplicates`: `15` прогонов
3. `ExtendibleBuild`, `ExtendibleGet`, `ExtendibleInsert`, `ExtendibleUpdate`, `ExtendibleDelete`: `5` прогонов


1. `runs` — число независимых прогонов benchmark'а;
2. `mean` — среднее значение по прогонам;
3. `stdev` — стандартное отклонение по прогонам.

## Результаты: Perfect Hash

### BenchmarkPerfectBuild

| Size | Runs | Mean items/s | Mean ns/item | Stdev ns/item |
|---|---:|---:|---:|---:|
| `10000` | `10` | `2,117,018` | `514.02` | `146.75` |
| `50000` | `10` | `1,922,842` | `583.02` | `197.80` |
| `100000` | `10` | `1,665,031` | `610.73` | `87.66` |
| `250000` | `10` | `1,771,119` | `585.89` | `119.79` |

Графики:

![BenchmarkPerfectBuild](artifacts/benchmarks/perfect/BenchmarkPerfectBuild/plot.png)

Профили:

![BenchmarkPerfectBuild CPU profile](artifacts/profiles/perfect/BenchmarkPerfectBuild/cpu.png)
![BenchmarkPerfectBuild MEM profile](artifacts/profiles/perfect/BenchmarkPerfectBuild/mem.png)

### BenchmarkPerfectGet

| Size | Runs | Mean ops/s | Mean ns/op | Stdev ns/op |
|---|---:|---:|---:|---:|
| `10000` | `15` | `21,141,801` | `49.35` | `11.54` |
| `50000` | `15` | `16,249,017` | `74.74` | `35.01` |
| `100000` | `15` | `4,479,100` | `230.11` | `40.26` |
| `250000` | `15` | `2,788,784` | `359.62` | `19.75` |

Графики:

![BenchmarkPerfectGet](artifacts/benchmarks/perfect/BenchmarkPerfectGet/plot.png)

Профили:

![BenchmarkPerfectGet CPU profile](artifacts/profiles/perfect/BenchmarkPerfectGet/cpu.png)
![BenchmarkPerfectGet MEM profile](artifacts/profiles/perfect/BenchmarkPerfectGet/mem.png)

### Вывод по Perfect Hash

1. Построение близко к линейному по числу ключей.
2. `Get` очень быстрый и без аллокаций, но на больших таблицах заметно влияние cache locality.
3. Основной компромисс классический: дорогое построение в обмен на очень быстрый lookup.

## Результаты: LSH

### BenchmarkLSHBuild

| Size | Runs | Mean docs/s | Mean ns/doc | Stdev ns/doc |
|---|---:|---:|---:|---:|
| `10000` | `10` | `117,355` | `8583.80` | `764.53` |
| `50000` | `10` | `142,391` | `7103.80` | `758.58` |
| `100000` | `10` | `151,628` | `6650.70` | `660.87` |
| `250000` | `10` | `154,155` | `6493.30` | `214.28` |

Графики:

![BenchmarkLSHBuild](artifacts/benchmarks/lsh/BenchmarkLSHBuild/plot.png)

Профили:

![BenchmarkLSHBuild CPU profile](artifacts/profiles/lsh/BenchmarkLSHBuild/cpu.png)
![BenchmarkLSHBuild MEM profile](artifacts/profiles/lsh/BenchmarkLSHBuild/mem.png)

### BenchmarkLSHAdd

| Size | Runs | Mean docs/s | Mean ns/doc | Stdev ns/doc |
|---|---:|---:|---:|---:|
| `10000` | `10` | `107,451` | `9581.90` | `1826.96` |
| `50000` | `10` | `157,502` | `6409.60` | `659.62` |
| `100000` | `10` | `159,769` | `6296.50` | `549.76` |
| `250000` | `10` | `92,396` | `10930.40` | `1148.30` |

Графики:

![BenchmarkLSHAdd](artifacts/benchmarks/lsh/BenchmarkLSHAdd/plot.png)

Профили:

![BenchmarkLSHAdd CPU profile](artifacts/profiles/lsh/BenchmarkLSHAdd/cpu.png)
![BenchmarkLSHAdd MEM profile](artifacts/profiles/lsh/BenchmarkLSHAdd/mem.png)

### BenchmarkLSHFindDuplicates

| Size | Runs | Mean ops/s | Mean ns/op | Stdev ns/op |
|---|---:|---:|---:|---:|
| `10000` | `15` | `155,027` | `9478.00` | `5374.06` |
| `50000` | `15` | `16,010` | `67760.27` | `20683.79` |
| `100000` | `15` | `7,231` | `143619.40` | `29071.59` |
| `250000` | `15` | `3,745` | `268051.93` | `16833.09` |

Графики:

![BenchmarkLSHFindDuplicates](artifacts/benchmarks/lsh/BenchmarkLSHFindDuplicates/plot.png)

Профили:

![BenchmarkLSHFindDuplicates CPU profile](artifacts/profiles/lsh/BenchmarkLSHFindDuplicates/cpu.png)
![BenchmarkLSHFindDuplicates MEM profile](artifacts/profiles/lsh/BenchmarkLSHFindDuplicates/mem.png)

### BenchmarkLSHFullScanDuplicates

| Size | Runs | Mean ops/s | Mean ns/op | Stdev ns/op |
|---|---:|---:|---:|---:|
| `10000` | `10` | `934` | `1094618.00` | `165047.71` |
| `50000` | `10` | `304` | `3444945.40` | `734186.70` |
| `100000` | `10` | `166` | `6115525.50` | `722007.13` |
| `250000` | `10` | `74` | `13644190.70` | `1335304.52` |

Графики:

![BenchmarkLSHFullScanDuplicates](artifacts/benchmarks/lsh/BenchmarkLSHFullScanDuplicates/plot.png)

Профили:

![BenchmarkLSHFullScanDuplicates CPU profile](artifacts/profiles/lsh/BenchmarkLSHFullScanDuplicates/cpu.png)
![BenchmarkLSHFullScanDuplicates MEM profile](artifacts/profiles/lsh/BenchmarkLSHFullScanDuplicates/mem.png)

### Вывод по LSH

1. `Build` и `Add` ведут себя близко к линейному росту по объему корпуса.
2. `FindDuplicates` на всех размерах на порядки быстрее полного сканирования.
3. На `250000` документов `FindDuplicates` дает около `3745 ops/s`, а `FullScanDuplicates` только `73.89 ops/s`, то есть ускорение примерно в `50.7x`.
4. Цена ускорения — более тяжелое построение и заметное потребление памяти под индекс и band buckets.

## Результаты: Extendible Hashing

### BenchmarkExtendibleBuild
| Size | Limit | Runs | Mean ops/s | Mean ns/item |
|---|---:|---:|---:|---:|
| `10000` | `32` | `5` | `2,332,611` | `512.40` |
| `10000` | `64` | `5` | `2,827,500` | `396.32` |
| `10000` | `128` | `5` | `2,229,382` | `471.82` |
| `50000` | `32` | `5` | `2,193,270` | `489.08` |
| `50000` | `64` | `5` | `3,071,262` | `361.52` |
| `50000` | `128` | `5` | `2,180,702` | `482.14` |
| `100000` | `32` | `5` | `2,700,726` | `417.32` |
| `100000` | `64` | `5` | `2,976,103` | `360.54` |
| `100000` | `128` | `5` | `2,103,622` | `481.46` |
| `250000` | `32` | `5` | `2,085,818` | `494.20` |
| `250000` | `64` | `5` | `3,073,043` | `366.58` |
| `250000` | `128` | `5` | `2,734,712` | `377.50` |

Графики:

![BenchmarkExtendibleBuild](artifacts/benchmarks/extendible/BenchmarkExtendibleBuild/plot.png)

Профили:

![BenchmarkExtendibleBuild CPU profile](artifacts/profiles/extendible/BenchmarkExtendibleBuild/cpu.png)
![BenchmarkExtendibleBuild MEM profile](artifacts/profiles/extendible/BenchmarkExtendibleBuild/mem.png)

### BenchmarkExtendibleGet

| Size | Limit | Runs | Mean ops/s | Mean ns/op |
|---|---:|---:|---:|---:|
| `10000` | `32` | `5` | `4,597,182` | `226.88` |
| `10000` | `64` | `5` | `4,642,690` | `216.52` |
| `10000` | `128` | `5` | `7,756,018` | `136.05` |
| `50000` | `32` | `5` | `3,223,048` | `328.84` |
| `50000` | `64` | `5` | `3,504,129` | `299.86` |
| `50000` | `128` | `5` | `3,501,444` | `290.72` |
| `100000` | `32` | `5` | `1,744,763` | `634.66` |
| `100000` | `64` | `5` | `2,118,767` | `478.58` |
| `100000` | `128` | `5` | `3,012,259` | `345.46` |
| `250000` | `32` | `5` | `792,304` | `1341.14` |
| `250000` | `64` | `5` | `1,508,814` | `671.84` |
| `250000` | `128` | `5` | `2,112,258` | `506.18` |

Графики:

![BenchmarkExtendibleGet](artifacts/benchmarks/extendible/BenchmarkExtendibleGet/plot.png)

Профили:

![BenchmarkExtendibleGet CPU profile](artifacts/profiles/extendible/BenchmarkExtendibleGet/cpu.png)
![BenchmarkExtendibleGet MEM profile](artifacts/profiles/extendible/BenchmarkExtendibleGet/mem.png)

### BenchmarkExtendibleInsert

| Size | Limit | Runs | Mean ops/s | Mean ns/item |
|---|---:|---:|---:|---:|
| `10000` | `32` | `5` | `2,420,730` | `439.08` |
| `10000` | `64` | `5` | `3,540,426` | `365.10` |
| `10000` | `128` | `5` | `2,072,892` | `494.32` |
| `50000` | `32` | `5` | `2,144,724` | `564.70` |
| `50000` | `64` | `5` | `2,122,045` | `499.90` |
| `50000` | `128` | `5` | `1,948,118` | `545.10` |
| `100000` | `32` | `5` | `1,759,437` | `618.40` |
| `100000` | `64` | `5` | `1,998,049` | `583.12` |
| `100000` | `128` | `5` | `2,589,829` | `402.02` |
| `250000` | `32` | `5` | `1,061,746` | `943.60` |
| `250000` | `64` | `5` | `2,335,245` | `442.94` |
| `250000` | `128` | `5` | `2,532,186` | `395.96` |

Графики:

![BenchmarkExtendibleInsert](artifacts/benchmarks/extendible/BenchmarkExtendibleInsert/plot.png)

Профили:

![BenchmarkExtendibleInsert CPU profile](artifacts/profiles/extendible/BenchmarkExtendibleInsert/cpu.png)
![BenchmarkExtendibleInsert MEM profile](artifacts/profiles/extendible/BenchmarkExtendibleInsert/mem.png)

### BenchmarkExtendibleUpdate

| Size | Limit | Runs | Mean ops/s | Mean ns/item |
|---|---:|---:|---:|---:|
| `10000` | `32` | `5` | `3,551,701` | `284.50` |
| `10000` | `64` | `5` | `6,634,476` | `170.14` |
| `10000` | `128` | `5` | `4,111,264` | `255.42` |
| `50000` | `32` | `5` | `5,121,460` | `213.00` |
| `50000` | `64` | `5` | `5,814,578` | `173.70` |
| `50000` | `128` | `5` | `3,487,437` | `300.54` |
| `100000` | `32` | `5` | `3,745,606` | `271.02` |
| `100000` | `64` | `5` | `4,420,751` | `239.54` |
| `100000` | `128` | `5` | `3,480,015` | `311.54` |
| `250000` | `32` | `5` | `4,791,320` | `214.28` |
| `250000` | `64` | `5` | `6,378,071` | `162.36` |
| `250000` | `128` | `5` | `5,308,497` | `190.46` |

Графики:

![BenchmarkExtendibleUpdate](artifacts/benchmarks/extendible/BenchmarkExtendibleUpdate/plot.png)

Профили:

![BenchmarkExtendibleUpdate CPU profile](artifacts/profiles/extendible/BenchmarkExtendibleUpdate/cpu.png)
![BenchmarkExtendibleUpdate MEM profile](artifacts/profiles/extendible/BenchmarkExtendibleUpdate/mem.png)

### BenchmarkExtendibleDelete

| Size | Limit | Runs | Mean ops/s | Mean ns/item |
|---|---:|---:|---:|---:|
| `10000` | `32` | `5` | `2,649,118` | `404.96` |
| `10000` | `64` | `5` | `4,433,591` | `243.12` |
| `10000` | `128` | `5` | `3,018,120` | `428.34` |
| `50000` | `32` | `5` | `1,845,121` | `547.98` |
| `50000` | `64` | `5` | `3,520,767` | `296.34` |
| `50000` | `128` | `5` | `4,047,635` | `260.78` |
| `100000` | `32` | `5` | `2,776,551` | `390.92` |
| `100000` | `64` | `5` | `3,697,144` | `298.18` |
| `100000` | `128` | `5` | `2,721,706` | `376.26` |
| `250000` | `32` | `5` | `2,667,344` | `382.24` |
| `250000` | `64` | `5` | `5,086,829` | `204.12` |
| `250000` | `128` | `5` | `4,646,598` | `224.30` |

Графики:

![BenchmarkExtendibleDelete](artifacts/benchmarks/extendible/BenchmarkExtendibleDelete/plot.png)

Профили:

![BenchmarkExtendibleDelete CPU profile](artifacts/profiles/extendible/BenchmarkExtendibleDelete/cpu.png)
![BenchmarkExtendibleDelete MEM profile](artifacts/profiles/extendible/BenchmarkExtendibleDelete/mem.png)


### Вывод по Extendible Hashing

1. Для `Build`, `Get`, `Insert`, `Update`, `Delete` конфигурация `limit=64` или `limit=128` почти всегда лучше `limit=32` на больших размерах.
2. Наиболее плохой сценарий для `Get` — `limit=32`, `size=250000`: `792,304 ops/s`.
3. Лучший `Get` на том же размере — `limit=128`: `2,112,258 ops/s`, то есть примерно в `2.67x` быстрее.
4. `Sync` все равно дорогой.
5. `Update` и `Delete` заметно дешевле `Insert`, потому что не требуют такого же числа split-операций и роста структуры.

## Профилирование CPU и памяти

### Анализ профилей

#### Perfect Hash

1. CPU в `Build` уходит в построение таблицы, работу с secondary buckets и копирование данных.
2. CPU в `Get` почти полностью сосредоточен в самой lookup-операции.
3. Память в основном расходуется на построение структуры, а не на поиск.

#### LSH

1. Основные затраты CPU приходятся на построение signature и на работу со структурами индекса.
2. Основной memory hotspot — `Index.Add`, потому что именно в этой операции формируются shingles, signature и band buckets.
3. `FindDuplicates` заметно дешевле полного сканирования по времени, но все равно создает много временных структур.

#### Extendible Hashing

1. Для всех операций доминирует `linux.Syscall6`, то есть стоимость системных вызовов и работы с файлами.
2. Основные memory hotspots в профиле — `splitBucket` и чтение `Bucket.Entries`, потому что split и обход bucketов приводят к перераспределению и материализации записей.
3. Это подтверждает, что реализация ограничена не столько pure CPU-логикой, сколько файловой подсистемой и схемой синхронизации.
