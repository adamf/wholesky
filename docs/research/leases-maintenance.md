# Aircraft leases, ownership and maintenance: what equipment really costs an airline

Research note for parameterising the cost side of an airline simulator. Compiled 18 November 2026.

## How to read this note

Every number carries one of three provenance marks.

- **[S]** fetched from a named source in this session (URL in the Sources list at the end).
- **[R]** recalled from published reports and trade press, **not re-verified in this session**. Treat as a good prior, check before quoting.
- **ESTIMATE** my own interpolation or scaling. Not a published figure.

**Research limits.** The session's shared web-search budget ran out after about 22 searches, so sections 2, 3 and 7 and much of section 6 rest on direct fetches of known pages plus recalled figures. Three sources asked for could not be read: the FAA "Economic Values" Section 4 PDF (HTTP 403), the full IATA MCX report (the public version stops at page 21, before the narrowbody/widebody split) and the IATA lease guidance PDF (404). Aviation Strategy's AVAC tables were read from the public part of each page, and the extraction lost column alignment on some rows; rows marked "(bands inferred)" need checking against the PDF.

Appraisers disagree by 5-15% on values and lease rates. AVAC (via Aviation Strategy) runs higher than Cirium Ascend and IBA, especially on lease rates for widebodies. Mixing appraisers across dates exaggerates or hides cycle moves; the time series in section 1 note the appraiser for each point.

---

## Master table by aircraft type

Basis: 2024-26 market unless a year is given. US$. Lease rates are monthly dry operating lease rent. "n/a" in the new columns means the type is out of production. Reserves are whole-aircraft totals (both engines, airframe, gear, APU) expressed per flight hour plus per flight cycle. DMC is direct maintenance cost at mature age (8-12 years), per block hour. ACMI is per block hour, summer, ex-fuel.

| Type | IATA / ICAO | Typical seats | Market value new (US$m) | Lease new (US$k/mo) | Lease at 10-12 yrs (US$k/mo) | Reserves per FH + per FC | DMC per BH | C-check: interval / cost / days | Heavy check: interval / cost / days | Engine shop visit: cost / interval | ACMI per BH |
|---|---|---|---|---|---|---|---|---|---|---|---|
| ATR 72-600 | AT7 / AT76 | 70-72 | 20-22 [S] (list 26.8, ~25% discount [S]) | 130 in 2018 [S]; 140-160 ESTIMATE | 100-115 [S] | 170-230 + 60-90 ESTIMATE | 500-650 ESTIMATE | 5,000 FH [S] / 0.12-0.25m ESTIMATE / 5-8 | 2/4/8-yr structural [S] / 0.5m (2018) [S] / 14-21 | PW127: 0.3-1.0m [S]; HSI ~3.5-4k FH, overhaul ~7-8k FH [R] | 1,300-1,800 ESTIMATE |
| Dash 8-400 | DH4 / DH8D | 76-78 | n/a; last new ~20-22 ESTIMATE | n/a; 150-170 ESTIMATE | 80-110 ESTIMATE | 230-300 + 70-100 ESTIMATE | 700-850 ESTIMATE | 8,000 FH [S] / 0.2-0.35m ESTIMATE / 7-10 | 8/12-yr structural / 0.6-0.9m ESTIMATE / 14-21 | PW150A: 1.0-1.5m ESTIMATE / 8-10k EFH | 1,500-2,000 ESTIMATE |
| CRJ-900 | CR9 / CRJ9 | 76-90 | n/a; used "nearing 10" [S] | n/a; 180-200 ESTIMATE | 80-110 ESTIMATE | 260-340 + 100-140 ESTIMATE | 850-1,100 ESTIMATE (700-940/FH in 2009 [S]) | 8,000 FH [S] / 0.25-0.4m ESTIMATE / 7-12 | 8-12 yr / 0.7-1.0m ESTIMATE / 21-28 | CF34-8C5: 1.3-2.0m ESTIMATE / 10-14k EFH | 1,800-2,400 ESTIMATE |
| E175 | E75 / E75L | 76-88 | 28-31 ESTIMATE (26.8 in 2019 [S]) | 200-230 ESTIMATE (182 in 2019 [S]) | ~120 [S] | 250-330 + 100-140 ESTIMATE | 800-1,000 ESTIMATE (771-837/FH in 2009 [S]) | 7,500 FH [S] / 0.25-0.4m ESTIMATE / 7-10 | 8-10 yr / 0.7-1.0m ESTIMATE / 21-28 | CF34-8E: 1.3-2.0m ESTIMATE / 10-14k EFH | 1,900-2,500 ESTIMATE |
| E190-E2 / E195-E2 | 290, 295 / E290, E295 | 97-114 / 132-146 | ~34 / ~38 [S] | ~270 ESTIMATE / ~300 [S] | n/a (EIS 2018); E190-E1 at 10 yrs 130-160 ESTIMATE | 280-360 + 130-180 ESTIMATE | 750-950 ESTIMATE | 10,000 FH [S] / 0.25-0.4m ESTIMATE / 7-10 | none yet; 0.8-1.1m ESTIMATE | PW1900G: 2.5-4.5m ESTIMATE; early removals | 2,000-2,800 ESTIMATE (E1) |
| A220-100 / -300 | 221, 223 / BCS1, BCS3 | 109-125 / 137-145 | 33-35 ESTIMATE / just above 40 [S] | 240-260 ESTIMATE / ~300 [S] | n/a (EIS 2016); 170-200 ESTIMATE | 300-380 + 150-200 ESTIMATE | 800-1,000 ESTIMATE | 8,500 FH [S] / 0.3-0.45m ESTIMATE / 7-10 | 12 yr / 1.0-1.4m ESTIMATE / 21-28 | PW1500G: 2.5-4.5m ESTIMATE | 2,600-3,300 ESTIMATE |
| A319ceo | 319 / A319 | 138-150 | n/a (35.0 in 2019, 25.9 in 2020 [S], bands inferred) | n/a (259 in 2019 [S]) | 150 (2019), 112 (2020) [S]; 140-180 ESTIMATE now | 380-480 + 320-480 ESTIMATE | 950-1,150 ESTIMATE | as A320 | as A320 | as A320ceo | 2,200-3,000 ESTIMATE |
| A320ceo | 320 / A320 | 174-180 | n/a (42.8 in 2019, 32.5 in 2020 [S]) | n/a (~300 in 2018 [S]) | 230-250 (Sep 2024) [S] | 380-480 + 320-480 ESTIMATE; worked example in s.3 [S] | 1,000-1,250 ESTIMATE (982-1,020/FH in 2006 [S]) | 36 mo / 12,000 FH / 8,000 FC [S] / 0.3-0.6m ESTIMATE / 5-10 | 6 yr: 0.8-1.3m / 21-28; 12 yr: 1.5-2.5m / 35-55 ESTIMATE | CFM56-5B: 1.8-2.5m PR, >3.5m with LLPs (broker [S]); 3-6m+ [R]. V2500: 2.25-3.63m (2013) [S]. First run 20-30k EFH [S], mature 10-15k | 2,500-4,000 ESTIMATE |
| A321ceo | 321 / A321 | 200-220 | n/a (48.4 in 2019, 35.1 in 2020 [S]) | n/a (344 in 2019 [S]) | 157 (2020) [S]; 250-300 ESTIMATE now | 420-520 + 380-560 ESTIMATE | 1,150-1,400 ESTIMATE | as A320 | as A320, +10-15% | as A320ceo; high-thrust variants ~15% dearer [S] | 3,000-4,500 ESTIMATE |
| A320neo | 32N / A20N | 180-186 | ~55 ESTIMATE (49 in 2018, 52.0 in 2019 [S]) | 400 (Sep 2024), 415-418 (2026) [S] | n/a (EIS 2016); 300-330 ESTIMATE | 400-520 + 420-600 ESTIMATE | 700-900 ESTIMATE (honeymoon) | as A320 | as A320 | LEAP-1A: 2.0-4.5m+, 60-120+ days (broker [S]). PW1100G: 2.5-5.0m+, 70-140+ days; powder-metal visits 250-300 days [S] | 3,000-4,500 ESTIMATE |
| A321neo / LR / XLR | 32Q / A21N | 200-239 (XLR 160-200) | 63-66 / 66-70 / 72-78 ESTIMATE (58.3, LR 62.2 in 2019 [S]) | 460 (Sep 2024), 455-460 (2026) [S]; LR/XLR 480-540 ESTIMATE | n/a; 350-380 ESTIMATE | 440-560 + 450-650 ESTIMATE | 800-1,000 ESTIMATE | as A320 | as A320, +10-15% | as A320neo | 3,500-5,000 ESTIMATE |
| 737-700 | 73G / B737 | 137-143 | n/a (~35, 2008-18 [S]) | n/a | 133 (2019), 105 (2020) [S], bands inferred; 140-180 ESTIMATE now | 360-460 + 300-450 ESTIMATE | 950-1,150 ESTIMATE | as 737-800 | as 737-800 | CFM56-7B, as 737-800 | 2,300-3,000 ESTIMATE |
| 737-800 | 738 / B738 | 162-189 | n/a (48.3 in 2017, 44.9 in 2019, 33.1 in 2020 [S]) | n/a (339 in 2019 [S]) | 230-250 (Sep 2024) [S] | 380-480 + 320-480 ESTIMATE (2010-13 components in s.3 [S]) | 1,000-1,250 ESTIMATE (909-1,128/FH in 2010 [S]) | 24 mo / 7,500 FH / 4,000 FC [S] / 0.2-0.5m ESTIMATE / 5-10 | 8-12 yr / 1.0m actual 2018 [S]; 1.2-2.0m ESTIMATE now / 21-35 | CFM56-7B: 1.2-1.6m core (2008) [S]; 1.8-2.5m PR, >3.5m with LLPs (broker [S]); 3-6m+ [R]. First run >20k EFH [S] | 2,500-4,000 ESTIMATE |
| 737 MAX 8 | 7M8 / B38M | 162-189 | ~55 (Sep 2024) [S] | 400 (Sep 2024), 415-418 (2026) [S] | n/a (EIS 2017); 300-330 ESTIMATE | 400-520 + 420-600 ESTIMATE | 700-900 ESTIMATE | as 737-800 (36 mo on later MPD [R]) | as 737-800 | LEAP-1B: 2.0-4.5m+ (broker [S]) | 3,000-4,500 ESTIMATE |
| 757-200 | 752 / B752 | 180-200 | n/a (5.7 in 2019, 5.0 in 2020 at 20 yrs [S]) | n/a | at 20+ yrs: 80-140 ESTIMATE | 600-800 + 500-700 ESTIMATE | 1,600-2,000 ESTIMATE | 18-24 mo / 0.5-0.9m ESTIMATE / 10-14 | 6/12 yr / 2-3m ESTIMATE / 28-42 | RB211-535 / PW2000: 3.5-6m ESTIMATE | 4,000-5,500 ESTIMATE |
| 767-300ER | 763 / B763 | 211-269 | n/a (22.6 / 13.4 in 2019 [S], bands inferred) | n/a | 202-238 (2019), 144-162 (2020) [S]; 180-260 ESTIMATE now | 750-950 + 700-900 ESTIMATE | 1,900-2,400 ESTIMATE | 18 mo / 6,000 FH / 3,000 FC [S] / 0.6-1.0m ESTIMATE / 10-14 | 6 yr / 2.5-3.5m ESTIMATE / 28-42 | CF6-80C2 / PW4000: 3.5-5.5m ESTIMATE | 5,000-7,000 ESTIMATE |
| A330-200 | 332 / A332 | 240-290 | n/a (75.0 in 2019, 47.8 in 2020 [S]) | n/a (621 in 2019 [S]) | 239 (2020) [S]; 250-300 ESTIMATE now | 850-1,100 + 900-1,200 ESTIMATE | 1,800-2,300 ESTIMATE (1,085-1,426/FH in 2001 [S]) | 24-36 mo / 0.6-1.0m ESTIMATE / 10-14 | 6 yr: 2.0-2.8m; 12 yr: 3-4.5m ESTIMATE / 28-50 [intervals S] | Trent 700 / CF6-80E1 / PW4168: 3-5m (broker [S]); 5-8m with LLPs [R] / 18-25k EFH | 6,000-8,500 ESTIMATE |
| A330-300 | 333 / A333 | 290-300 | n/a (86.8 in 2019, 55.8 in 2020 [S]) | n/a (729 in 2019 [S]) | 330 at 12 yrs (Sep 2024) [S] | as A330-200 | as A330-200 | as A330-200 | as A330-200 | as A330-200 | 6,500-9,000 ESTIMATE |
| A330-900 | 339 / A339 | 287-300 | 110-120 ESTIMATE (115 in 2018, 112.6 in 2019 [S]) | 600-900 (AVITAS 2025) [S] | n/a (EIS 2018) | 800-1,000 + 900-1,200 ESTIMATE | 1,500-1,900 ESTIMATE | 36 mo / 0.6-1.0m ESTIMATE / 10-14 | 6/12 yr / 2-4m ESTIMATE | Trent 7000: flight-hour contract, 350-450/EFH per engine ESTIMATE | 7,000-9,500 ESTIMATE |
| 787-8 | 788 / B788 | 230-250 | 115-125 ESTIMATE (118.8 in 2019 [S]) | 800-900 ESTIMATE (861 in 2019 [S]) | 360 (2020) [S]; 450-550 ESTIMATE now | 850-1,100 + 1,000-1,400 ESTIMATE | 1,500-1,900 ESTIMATE | 36 mo [S] / 0.6-1.0m ESTIMATE / 7-12 | 12 yr [S] / 3-4.5m ESTIMATE / 28-42 | GEnx-1B / Trent 1000: 6-10m ESTIMATE; TAT up to ~200 days in 2026 [S] | 8,000-11,000 ESTIMATE |
| 787-9 | 789 / B789 | 280-300 | 150-160 ESTIMATE (145 in 2018, 146.9 in 2019 [S]) | 1,050 (Sep 2024) [S] | n/a (EIS 2014); 650-750 ESTIMATE | 900-1,150 + 1,100-1,500 ESTIMATE | 1,600-2,000 ESTIMATE | as 787-8 | as 787-8 | as 787-8 | 9,000-12,000 ESTIMATE |
| 787-10 | 781 / B78X | 318-336 | 160-170 ESTIMATE (154.2 in 2019 [S]) | 1,150-1,250 ESTIMATE (1,267 in 2019 [S]) | n/a (EIS 2018) | as 787-9 | 1,700-2,100 ESTIMATE | as 787-8 | as 787-8 | as 787-8 | n/a |
| A350-900 | 359 / A359 | 300-325 | 168.5 (2026), 151.4 (2022) [S] | 1,140 (Sep 2024) [S]; ~1,230 (2026, LRF 0.73%) [S] | n/a (EIS 2015); 750-850 ESTIMATE | 900-1,150 + 1,100-1,500 ESTIMATE | 1,500-1,900 ESTIMATE | 24-36 mo [S] / 0.6-1.0m ESTIMATE / 7-12 | 12 yr; gear also 12 yr [S] / 3-4.5m ESTIMATE / 28-42 | Trent XWB-84: 7-10m ESTIMATE; TAT up to ~200 days [S] | 10,000-13,000 ESTIMATE |
| A350-1000 | 351 / A35K | 350-370 | 185-195 ESTIMATE (167.9 in 2019 [S]) | 1,350-1,450 ESTIMATE (1,439 in 2019 [S]) | n/a (EIS 2018) | 1,000-1,300 + 1,300-1,700 ESTIMATE | 1,900-2,300 ESTIMATE | as A350-900 | as A350-900 | Trent XWB-97: 9-13m ESTIMATE; short time on wing in hot, sandy operations [R] | n/a |
| 777-200ER | 772 / B772 | 270-317 | n/a (2007 build worth ~30 in 2017; oldest 5-6 [S]) | n/a | at 20 yrs: 150-250 ESTIMATE | 1,000-1,300 + 1,400-1,800 ESTIMATE | 2,400-3,000 ESTIMATE (1,710-2,100/FH in 2008 [S]) | 24 mo / 0.8-1.3m ESTIMATE / 10-14 | 8 yr [S] / 4.0m actual 2018 [S] / 35-50 | GE90-94B / Trent 800 / PW4090: 6-10m ESTIMATE | 8,000-10,500 ESTIMATE |
| 777-300ER | 77W / B77W | 350-365 | n/a for most buyers (165.2 in 2015, 92.3 in 2020 [S]; United paid ~130 in 2015 [S]) | n/a (1,567 in 2015, 828 in 2020 [S]) | 450 at 12 yrs (Sep 2024) [S] | 1,000-1,300 + 1,700-2,100 ESTIMATE | 2,400-2,900 ESTIMATE | 24-36 mo / 0.8-1.3m ESTIMATE / 10-14 | 8 yr, three in a life [S] / 4.5m actual 2018 [S] / 35-50 | GE90-115B: 6-9m (broker [S]); 10-15m with LLPs [R] / 20-25k EFH first run | 9,500-13,000 ESTIMATE |
| 777-9 | 779 / B779 | ~426 | appraiser pre-delivery 178.1 (2019), 149.0 (2020) [S]; list 442.2 (2019) [R] | pre-delivery 1,483 (2019), 1,175 (2020) [S] | n/a; no deliveries, first slipped to 2027 [R] | n/a | n/a | n/a | n/a | GE9X: n/a | n/a |
| 747-400 | 744 / B744 | 416 | n/a (7.4 in 2019, 4.3 in 2020, conversion feedstock [S]) | n/a | 143-197 in 2015 [S] | 1,800-2,400 + 2,000-2,800 ESTIMATE | 3,500-4,500 ESTIMATE | 7,500 FH (-400) [S] / 1.0-1.5m ESTIMATE / 14 | 6 yr systems; 8/8/6 yr structures [S] / 6.0m actual 2018 [S] / 45-60 | CF6-80C2 / PW4056 / RB211-524: 3.5-5.5m ESTIMATE x4 | 8,000-12,000 ESTIMATE (freighter) |
| 747-8I | 74H / B748 | 364-467 | n/a (160.8 in 2015, 98.2 in 2019, 45.8 in 2020 [S]) | n/a (1,268 in 2015, 821 in 2019, 331 in 2020 [S]) | 5-yr-old: 276 (2020) [S] | 1,600-2,200 + 2,000-2,800 ESTIMATE | 3,000-3,800 ESTIMATE | 10,000 FH [S] / 1.0-1.5m ESTIMATE / 14 | as 747-400 | GEnx-2B: 5-8m ESTIMATE x4 | n/a |
| A380-800 | 388 / A388 | 480-520 | n/a (list 445.6 in 2018 [S]; 224.1 in 2015, 185.9 in 2019, 72.8 in 2020 [S]) | n/a (1,757 in 2015, 1,439 in 2019, 494 in 2020 [S]) | 758 (2019), 294 (2020) [S] | 2,200-3,000 + 2,500-3,500 ESTIMATE | 4,000-5,500 ESTIMATE | 24-36 mo / 1.5-2.5m ESTIMATE / 14-21 | 6 yr: 5-8m; 12 yr: 10-15m+ ESTIMATE / 45-60 | Trent 900 / GP7200: 6-9m ESTIMATE x4 | 18,000-25,000 ESTIMATE |

Seat counts and type codes are standard reference values [R].

---

## 1. Operating lease economics

### 1.1 List price against real price

Airbus stopped publishing list prices after 2018 and Boeing after 2019. Real prices sit 45-60% below list for ordinary orders and 60-70% below for launch customers and very large orders.

| Type | List price (US$m) | Appraised new value or reported deal (US$m) | Discount |
|---|---|---|---|
| A319 (2012) | 81 | 30 (WSJ/Ascend) [S] | 63% |
| 737-800 (2012) | 84 | 41 (WSJ/Ascend) [S] | 51% |
| A320neo (2018) | 110.6 [S] | 49 [S] | 56% |
| A319neo (2018) | 101.5 [S] | 39.3 (AVAC 2019) [S] | 61% |
| 737 MAX 8 (2019) | 121.6 [S] | 52.85 (2017), 54.5 (2018), ~55 (2024) [S] | 55% |
| 737 MAX 8, one campaign | 121.6 | 38, with the rival A321neo offer "upper 40s" (Leeham) [S] | 69% |
| 737 MAX 8-200, Ryanair Dec 2020 | ~110 (2014 order: over US$22bn for 200 [S]) | "one-third of the list price" for 75 more [S] | ~67% |
| 737-700, United Jan 2016 | ~80 | ~22 [S] | 73% |
| 787-8 | 248.3 (2019) [S] | "up to 46% lower" [S] | 46% |
| 787-9 | 292.5 (2019) [S] | 145 value (2018); Hawaiian paid under 100-115; production cost 80-90 [S] | 50-62% |
| 787-10 | 338.4 (2019) [S] | 154.2 (AVAC 2019) [S] | 54% |
| A350-900 | 317.4 (2018) [S]; 287.7 (2013) | 150.0 (AVAC 2019); 152 (Ascend 2013, 47% off) [S] | 47-53% |
| A350-1000 | 366.5 (2018) [S] | 167.9 (AVAC 2019) [S] | 54% |
| A330-800, TransAsia | 241.7 | 120 [S] | 50% |
| 777-300ER | 330 (2015), 375.5 (2019) [S] | United paid ~130 against a "normal" ~150 (Jan 2015) [S] | 55-61% |
| A380 | 445.6 (2018) [S] | 185.9 (AVAC 2019); the list "price tag" did not cover production cost [S] | 58% |
| ATR 72-600 | 26.8 [S] | 20.1 (2018) [S] | 25% |

Other list prices, recalled [R]: A220-100 81, A220-300 91.5, A321neo 129.5, A330-900 296.4 (Airbus 2018); 737-700 89.1, 737-800 106.1, MAX 10 134.9, 767-300ER 217.9, 777-9 442.2, 747-8I 418.4 (Boeing 2019). Turboprops and regional jets get smaller discounts (20-35%) because order sizes are small.

Engines are priced the same way. The PW1100G listed at US$12m in 2011 and cost Pratt about US$10m to build at the 2016 start of production, more than it sold for [S]. Engines are 25-26% of a 737 MAX's supplier cost (US$7-9m of US$28.5-35m) [S]. The engine maker recovers the discount through flight-hour service contracts and parts.

### 1.2 Lease rates through the cycle

Monthly rent, US$ thousands. AVAC figures are end-October each year.

| Type, age | 2015 | 2018-19 | 2020-21 trough | 2022 | 2024 | 2026 |
|---|---|---|---|---|---|---|
| A320neo, new | 386 (AVAC, pre-service) | 330-350 (AVN 2018); 368 (AVAC 2019) | ~284 (IBA Apr 2020, derived); 256 (AVAC Oct 2020) | up to 324 (IBA Jul) | 400 (IBA Sep) | 415-418 (Acumen) |
| A321neo, new | 478 (AVAC) | 406 (AVAC 2019) | 302 (AVAC) | 375 (Jul) | 460 (IBA Sep) | 455-460 (Acumen) |
| 737 MAX 8, new | 438 (AVAC) | >385 (2018); 411 (AVAC 2019) | ~263 (IBA Apr 2020, derived); 248 (AVAC) | 316 (IBA Jul) | 400 (IBA Sep) | 415-418 (Acumen) |
| 737-800, 10-12 yrs | n/a | 210 (Cirium, 2008 build, Jan 2020); 243 (AVAC 2019) | 139 (Cirium, Jan 2021); 131 (AVAC) | n/a | 230-250 (IBA Sep) | 286-297 for 2020 builds (Acumen) |
| A320ceo, 10 yrs | n/a | 200 (AVAC 2019) | 144 (AVAC) | n/a | 230-250 (IBA Sep) | as 737-800 |
| A220-300, new | n/a | 292 (AVAC 2019) | 218 (AVAC) | n/a | ~300 (IBA Jan 2025) | n/a |
| ATR 72-600, new | 170 at some point before 2018 (date not given) | 130 (AVN 2018) | n/a | n/a | used deals back above 100 (Cirium Nov 2024) | n/a |
| A330-900, new | n/a | 889 (AVAC 2019) | 571 (AVAC) | n/a | 600-900 (AVITAS 2025) | n/a |
| 787-9, new | 1,095 (AVAC) | 1,100 (AVAC 2019) | 724 (AVAC) | n/a | 1,050 (IBA Sep) | n/a |
| A350-900, new | 1,129 (AVAC) | 1,172 (AVAC 2019) | 789 (AVAC) | ~1,196 (Acumen, derived) | 1,140 (IBA Sep) | ~1,230 (Acumen, derived) |
| 777-300ER | new 1,567; 10 yrs 1,051 (AVAC) | n/a | new 828; 5 yrs 757 (AVAC) | n/a | 12 yrs: 450 (IBA Sep) | firming (Acumen) |
| A380, new | 1,757 (AVAC) | 1,439 (AVAC 2019) | 494 (AVAC) | n/a | n/a | n/a |
| 747-8I, new | 1,268 (AVAC) | 821 (AVAC 2019) | 331 (AVAC) | n/a | n/a | n/a |

All rows [S]. "Derived" means computed from a stated percentage change or lease rate factor.

What moved rates:

- **2015-19, cheap money.** Lease rate factors fell to 0.6% a month and as low as 0.55% as new lessors, many Chinese, bid for sale-and-leasebacks [S]. Rents on new narrowbodies drifted down although values held.
- **737 MAX grounding, March 2019 to November 2020.** Rent on a 10-year-old 737NG went from US$230,000 in January 2019 to US$300,000 in September 2019 [S].
- **COVID, 2020-21.** Cirium: 737-800 rents fell 32-42% during 2020 depending on age, A320ceo 25-40%; fleet-weighted, widebodies fell 25% and narrowbodies nearly 30% [S]. AVAC year on year: A320neo -30%, A321neo -26%, 787-9 -34%, A350-900 -33%, A330-900 -36%, 737-800 at 10 years -46%, 747-8I -60%, A380 -66% [S]. Four-engine types never recovered.
- **2022-26, shortage plus higher rates.** An Ascend note of August 2023 reported rises of up to 23% for A320ceo-family and 17% for 737NG rents in late Q2 [S]. By September 2024 new A320neo and MAX 8 rents reached US$400,000 and the A321neo US$460,000 [S]. Growth then slowed: SMBC's Q2 2025 note says single-aisle rents were still rising but more slowly than in 2023 and early 2024, while A330-900, 787-9 and A350-900 rents rose 7-12% from Q4 2023 [S]. Appraisers saw no rent difference between the A320neo and MAX 8 for six quarters [S].
- **Old widebodies came back.** A 12-year-old 777-300ER rented for US$450,000 in 2024 [S] and Acumen calls it the "indispensable bridge asset" of 2026 [S]. That is still less than half the US$1.05m a 10-year-old one fetched in 2015 [S].

### 1.3 Lease rate factors

Monthly rent divided by current market value, computed from the paired AVAC value and rent tables unless noted [S].

| Asset | 2015 | 2019 | 2020 | 2024-26 |
|---|---|---|---|---|
| A320neo, new | 0.80% | 0.71% (0.74% per AVN 2018) | 0.61% | ~0.73-0.76% ESTIMATE |
| 737 MAX 8, new | 0.80% | 0.77% | 0.58% | ~0.73-0.76% ESTIMATE |
| A350-900, new | 0.82% | 0.78% | 0.71% | 0.79% (2022), 0.73% (2026) per Acumen |
| 787-9, new | 0.81% | 0.75% | 0.67% | ~0.70% ESTIMATE |
| 777-300ER, new | 0.95% | n/a | 0.90% | n/a |
| A380, new | 0.78% | 0.77% | 0.68% | n/a |
| ATR 72-600, new | n/a | 0.65% (2018) | n/a | n/a |
| A320ceo, 10 yrs | n/a | 0.93% | 0.82% | n/a |
| 737-800, 10 yrs | n/a | 0.87% | 0.89% | n/a |
| A380, 10 yrs | n/a | 0.89% | 0.77% | n/a |
| 737-800, 20 yrs | n/a | 1.53% | 1.39% | n/a |

Rule of thumb that fits the data: 0.60-0.80% for new aircraft, 0.75-0.90% at 5 years, 0.85-1.00% at 10 years, 1.1-1.3% at 15 years, 1.4-1.6% and up at 20 years. The factor rises with age because the lessor must recover a faster-falling residual over a shorter lease.

Acumen's 2026 point matters for a simulator: values of scarce new aircraft rose faster than rents, so the factor on a new A350-900 **fell** from 0.79% to 0.73% between 2022 and 2026 even though interest rates rose [S]. Shortage shows up in value first, rent second.

A reference deal: a 12-year A320neo lease to a good credit at under US$370,000 a month, 0.74% of a US$49m aircraft, yields US$53m of rent, over US$8.5m of end-of-lease maintenance compensation, and an aircraft still worth about US$20m [S].

### 1.4 Value decay with age

From the AVAC tables [S]:

| Type and year | New | 5 yrs | 10 yrs | 20 yrs |
|---|---|---|---|---|
| A320ceo, 2019 (bands inferred) | 42.8 | 32.1 (75%) | 21.4 (50%) | ~7.9 (18%, 2020 table) |
| 737-800, 2019 (bands inferred) | 44.9 | 34.2 (76%) | 27.8 (62%) | 9.5 (21%) |
| 787-8, 2020 | 78.0 | 58.4 (75%) | 38.7 (50%) | n/a |
| 777-300ER, 2015 | 165.2 | 138.8 (84%) | 105.9 (64%) | n/a |
| A350-900, 2019 | 150.0 | 98.4 (66%) | n/a | n/a |
| 787-9, 2019 | 146.9 | 97.2 (66%) | n/a | n/a |
| A380, 2019 | 185.9 | 135.4 (73%) | 84.9 (46%) | n/a |

A 2003-build 737-700 fell from US$15.5m in 2016 to US$12m in 2018 [S]. A 2007-build 777-200ER that cost about US$130m was selling for about US$30m ten years later [S].

### 1.5 Terms, deposits, the lessors

- **Term.** New narrowbodies 8-12 years, new widebodies 12 years, extensions 2-6 years, used aircraft 3-8 years [R]. The A320neo reference deal above is 12 years [S]. BOC Aviation's average remaining term was 7.8 years across 451 owned aircraft averaging 5 years old at end-2025 [S].
- **Extensions.** Historically 50-60% of leases are extended at expiry. In 2025-26 some portfolios approach 80% as airlines bridge delivery delays [S].
- **Security deposit.** One to three months' rent, cash or letter of credit; strong credits pay none [R]. Wet leases usually take one month [S].
- **Rent.** Monthly in advance, fixed for the term or floating over a benchmark; fixed rent is set off the swap rate at delivery, so the delivery-date interest rate is locked for the whole term [R].
- **Net lease.** The lessee pays maintenance, insurance, taxes and all operating costs, and returns the aircraft in a contractual condition (section 3).
- **Share of fleet leased.** Under 2% in 1976, about 15% in the early 1990s, 25% in 2000, 40% in 2017 [S]. Cirium counted more than 51% of nearly 23,000 operational passenger aircraft as lessor owned or managed in 2022 [S]; AerCap says about 50% [S]. China is just under 60% [S]. Lessors took part in 62% of mid-life aircraft trades since 2000 [S].
- **World fleet.** 35,550 commercial aircraft in June 2025, of which 30,300 active and 5,250 stored; average age 15 years against 13 before COVID [S].

Largest lessors, 2026 (KPMG Aviation Leaders Report 2026 via Wikipedia) [S]:

| Lessor | Portfolio (aircraft) | Order backlog |
|---|---|---|
| AerCap | 1,734 | 278 |
| SMBC Aviation Capital | 783 | 227 |
| Avolon | 685 | 460 |
| Dubai Aerospace Enterprise | 622 | 74 |
| Air Lease Corporation | 592 | 216 |
| BBAM | 543 | 0 |
| ICBC Leasing | 488 | n/a |
| BOC Aviation | 468 | n/a |
| Carlyle Aviation Partners | 356 | n/a |
| Aviation Capital Group | 325 | n/a |

AerCap at 31 December 2025: 1,649 aircraft, over 1,200 engines, over 300 helicopters, 330 on order, about 300 customers, US$72bn of assets [S]. DAE closed a roughly US$9bn purchase of Macquarie AirFinance in July 2026 [S]. For scale in 2017: 150 lessors managed about 8,400 aircraft worth US$256bn [S].

### 1.6 Sale and leaseback

The airline orders in bulk at a deep discount, takes delivery, sells each aircraft to a lessor at or near market value and leases it back. The gap between the airline's price and the sale price is cash on delivery day. The airline then pays rent on the higher value for 8-12 years, so the gain is borrowed from future rent.

Using the evidence above: a MAX 8 bought at US$38-42m against a US$53-55m appraised value leaves US$10-15m an aircraft before the lessor's own margin. A realistic recognised gain is US$3-8m per narrowbody. ESTIMATE.

- IndiGo has ordered 100 (2005), 180 (2011, US$15bn list), 250 (2015, US$27bn), 300 (2019, US$25bn) and 500 (2023) A320-family aircraft [S] and financed most deliveries by sale and leaseback [R].
- Wizz Air leases nearly all its fleet through sale and leaseback [R].
- Ryanair does the opposite: it buys at the bottom of the cycle and keeps the aircraft. It handed back all leased 737s by July 2021 [S].
- United, once mostly an owner, sold and leased back 10 737-9s with DAE (July 2025) and 20 with SMBC (December 2025) [S].
- Spirit: of five sale-leasebacks in Q1 2024, two qualified as operating leases and three were "failed" sale-leasebacks kept on the balance sheet as financings [S].

### 1.7 Lease accounting

IFRS 16 took effect on 1 January 2019. It has no operating-lease category for lessees: every lease over 12 months becomes a right-of-use asset and a lease liability, and rent is replaced by depreciation plus interest. Leases of 12 months or less and assets worth under US$5,000 are exempt [S]. US GAAP's ASC 842 took effect for fiscal years starting after 15 December 2018; it also puts operating leases on the balance sheet but keeps a straight-line rent line in the income statement [S]. Under IFRS 16 a sale-and-leaseback gain is recognised only for the rights transferred to the lessor, which cut reported gains [R].

Simulator consequence: report rent as a cash cost, and show lease-adjusted debt (about 7 times annual rent was the pre-2019 analyst convention [R]) when judging an airline's leverage.

---

## 2. Other lease and financing types

### 2.1 Finance leases, EETCs, JOLCOs, export credit

- **Finance lease.** Full-payout, usually 10-12 years with a purchase option or balloon. The airline carries residual value risk and depreciates the aircraft. Delta's mainline fleet at 30 September 2026: 903 owned, 15 on finance lease, 87 on operating lease, of 1,005 [S].
- **EETC.** US airlines sell tranched certificates secured on a pool of aircraft. Senior tranches lend 35-55% of appraised value, junior tranches take it to 65-80%. A liquidity facility covers 18 months of interest. Protection rests on Section 1110 of the US Bankruptcy Code: within 60 days of filing the airline must cure defaults and agree to keep paying, or hand the aircraft back [R]. Coupons track Treasuries plus a spread. Recalled examples [R]: United 2019-2 class AA 2.70%; Delta 2020-1 AA 2.00%; American 2021-1 A 2.875%; United 2020-1 A 5.875% (October 2020, distress pricing); United 2023-1 A 5.80%; United 2024-1 AA 5.45% and A 5.875%. Range for a model: 2-3.5% in 2019-21 for senior paper, 5-6% in 2023-25, with 100-250 basis points more for junior tranches.
- **JOLCO.** Japanese operating lease with call option. Japanese equity investors put in 20-30% and take tax depreciation; the airline has a call option at year 10 or so. All-in cost runs below a straight bank loan for good credits. It depends on the yen: SMBC notes the JOL/JOLCO market is an important funding source and benefits when the yen weakens [S].
- **Export credit.** US Ex-Im, UK Export Finance, Bpifrance and Euler Hermes guarantee loans of up to 85% of net price over 12 years under the OECD Aircraft Sector Understanding, with a risk-based premium paid up front. Countries that have ratified the Cape Town Convention with the qualifying declarations get a discount on the premium [R].

### 2.2 Wet lease (ACMI), damp lease

The lessor supplies aircraft, crew, maintenance and insurance. The lessee pays a rate per block hour against a minimum guaranteed number of hours a month, plus fuel, airport and navigation charges, handling, catering, crew hotels and positioning flights. A damp lease is the same with the lessee's own cabin crew [S]. Wet leases run 1-24 months; dry leases more than two years [S]. The deposit is usually one month [S].

No published rate card was reachable. Two explainers use US$2,000 a block hour at 240 guaranteed hours and US$2,750 at 250 hours (US$687,500 a month) for an A320, both illustrative [S]. The ACMI column of the master table is my ESTIMATE from those and from recalled broker quotes: A320 and 737-800 at US$2,500-4,000, widebodies US$6,000-13,000. Typical guaranteed hours are 200-300 a month in summer and 100-150 in winter, with winter rates 20-40% lower [R].

Market facts [S]: summer 2024 was the peak for ACMI price and demand. 2025 was the strongest year by block hours. To end Q3 2026 global ACMI flying fell more than 15% against the same period of 2025 and Europe 23%; against 2025, A320ceo flying fell 37%, 737-800 17%, A321 38%, while widebody ACMI rose 7% (ACC Aviation, ch-aviation data). The ACMI market was projected at US$4.8bn in 2021 rising to US$7.7bn by 2028 [S]. Named operators: Air Belgium, CityJet, Euro Atlantic, GetJet, Hi Fly, SmartLynx, Titan, Wamos [S]. IndiGo wet-leased two 777s from Turkish Airlines and six 787s from Norse Atlantic (first flight 1 March 2025) to cover capacity it could not buy [S].

**Short summer sub-leases between carriers** exploit opposite seasons: Canadian leisure carriers send narrowbodies to European charter airlines in summer and take them back for winter. Terms are 4-6 months, dry or damp, at roughly the monthly rent plus maintenance reserves [R].

### 2.3 Power-by-the-hour leases

Rent becomes a rate per flight hour, often with a floor. In 2020-21 lessors accepted this from airlines in restructuring, usually for 6-24 months before reverting to fixed rent, and often in exchange for a longer term. Recalled cases [R]: Norwegian (from spring 2020 into 2021), LATAM, Avianca and Aeromexico during Chapter 11, Virgin Atlantic, Philippine Airlines. Outside a crisis, lessors resist it because it moves traffic risk onto them.

### 2.4 Spare engines

| Engine | Monthly rent | Source |
|---|---|---|
| CFM56-7B | US$75,000 (2019) to US$100,000 (2024) | IBA [S] |
| CFM56-5B/3 PIP | up 11% in 2025 | [S] |
| V2500-A5 | values down 7%, rents down 6% in 2025 | IBA [S] |
| PW1127G | US$95,000 (2019) to US$110,000 (2023); another source says more than double 2019 | [S] |
| LEAP-1A26 | over US$125,000 (2024) | IBA [S] |
| LEAP-1B | US$130,000-150,000 long term, US$6,000-7,000 a day short term (July 2025) | ISTAT appraisal [S] |

On top of rent the lessee pays use fees: roughly US$150-250 per engine flight hour for performance restoration and US$150-300 per cycle for life-limited parts on a narrowbody engine [R], consistent with the reserve rates in section 3. Short-term rent at US$6,000-7,000 a day is 40% above the long-term monthly rate [S].

About 430 LEAP-1B spares exist against a fleet of almost 4,200 engines, 10.3% [S]. Lessors buying several LEAP-1Bs get only 2-4% off [S]. LEAP-1A lease periods in 2025 were 179% longer than in 2023 [S].

**Green-time engines.** Rather than pay for a shop visit on an old CFM56 or V2500, an operator leases or buys a used engine with a few thousand cycles left and runs it to the limit. A V2500 removed early has "stub life" parts with 6,000-10,000 cycles remaining that are sold on [S]. The same logic applies to auxiliary power units: a shop visit quote above about US$250,000 on a 767 unit makes it beyond economic repair because a used unit costs less [S]. This is why scarce shop capacity raises the value of old aircraft: they are worth their engines.

**Engine values** [R]: new LEAP-1A/1B and PW1100G US$14-17m; a CFM56-7B or -5B fresh from overhaul US$8-11m (double 2019); GE90-115B over US$32m at full life and a Trent 700 about US$18m overhauled (broker figures [S]).

---

## 3. Maintenance reserves and return conditions

### 3.1 How reserves are set

A reserve rate is the expected cost of an event divided by the interval between events, in the unit that drives it: flight hours for engine performance restoration, cycles for life-limited parts and landing gear, calendar months for airframe structural checks, auxiliary power unit hours for the APU [S]. Reserves are paid monthly in arrears on reported use, escalate at 3% a year or more (life-limited parts follow the manufacturer's catalogue, lately 5-10% a year [R]), and are drawn down when the lessee presents invoices for a qualifying event [S]. No reserves accrue while the aircraft sits in a check [S].

Worked example, a US$45m narrowbody on a 12-year lease at 0.8% a month, flying 300 hours and 150 cycles a month [S]:

| Item | Event cost and interval | Rate | Per month |
|---|---|---|---|
| Rent | 0.8% of US$45m | n/a | US$360,000 |
| Engine performance restoration, x2 | US$1.875m per engine every 15,000 hours | US$125 per engine hour | US$75,000 |
| Engine life-limited parts, x2 | US$2.25m per engine per 15,000 cycles | US$150 per engine cycle | US$45,000 |
| Airframe 6-year and 12-year checks | n/a | calendar | US$20,000 |
| Landing gear | US$350,000 every 10 years | calendar | US$3,000 |
| APU | US$450,000 every 4 years | n/a | US$9,375 |
| Thrust reversers | US$600,000 every 10 years | calendar | US$5,000 |
| **Total reserves** | | US$525 per flight hour equivalent | **US$157,375** |

Reserves are 44% of rent in this example. That is the number to carry into a model: a lessee paying cash reserves hands over roughly 1.4 times the headline rent.

The engine figures in that example are pre-2020. Current parts prices put a CFM56 or V2500 at US$150-200 per hour and US$200-280 per cycle per engine, and a LEAP or GTF higher on life-limited parts. ESTIMATE.

Older published component rates, useful for proportions:

- **A320, 2006** [S]: line and ramp US$212/FH, base checks US$128/FH, heavy components US$120/FH, rotable component support US$180/FH; total with engines US$982-1,020/FH.
- **A320 base checks, 2011** [S]: US$168/FH in the first 12-year cycle, US$185/FH in the second, at US$50 per man-hour.
- **737NG, 2010** [S]: A-checks US$32/FH; base checks US$65-70/FH (US$2.5m per 39,000 FH cycle); wheels, brakes, gear, thrust reversers and APU US$254 per cycle (APU alone US$22); interior US$28/FH; paint US$5/FH; engines US$152-249 per engine hour each; total US$909-1,128/FH.
- **V2500-A5, 2013** [S]: two or three shop visits of US$2.25-3.63m across a 20,000-cycle parts life; total reserve including parts US$406-440 per cycle for the lower thrust variants, about US$600 for the V2530/33.
- **Landing gear** [S]: 737NG exchange US$320,000 at 10 years or 18,000 cycles. The calendar limit sets a minimum monthly rate whatever the aircraft flies.
- **APU** [S]: a heavy visit on the A320/737 unit costs about US$350,000, of which life-limited parts can be US$200,000; large widebody units US$500,000-700,000. Unmanaged, an APU costs up to US$100 per flight hour.
- **E-Jet, 2009** [S]: base check reserve US$79/FH. **ATR 72** [S]: base checks US$50/FH in the first cycle, US$80/FH in the second. **Q400** [S]: base checks US$50-53/FH.

### 3.2 Who pays, and how

- **Weak credits** pay cash reserves monthly. The lessor holds the cash, earns on it, and keeps the balance if the lessee defaults [S].
- **Strong credits** pay no reserves. They give a letter of credit, or nothing, and settle at lease end: **end-of-lease compensation** equal to the reserve rate times hours and cycles consumed since the last event [R]. The US$8.5m on the A320neo reference deal is this payment [S].
- **Mirror clauses.** A used aircraft arrives part-worn. The next lessee either gets a lessor contribution towards its first event for the life the previous operator used, or returns it in the same condition it arrived [R].
- For an asset-backed lessor, the cash profile is lumpy: outflows are larger and rarer than inflows, and engine events cause most of them. Landing gear and airframe checks are calendar driven and do not shift with utilisation; engines do [S].

### 3.3 Return conditions and redelivery

Typical conditions [R]:

- airframe fresh from a C-check, with the 6-year or 12-year check not falling due for a set period;
- each engine with 3,000-5,000 hours of life left to its next shop visit and every life-limited part with 3,000-4,000 cycles remaining, passing a borescope and power check;
- landing gear and APU with a minimum time since overhaul;
- lessor's paint, a full records set with back-to-birth traceability for life-limited parts, and the cabin in the delivery layout.

**Half-life** return means each component at the midpoint of its interval, with a cash adjustment either way. **Full-life** means the lessee pays to bring everything to zero time, usually in cash.

Costs. Reconfiguring an A330-300 cabin at transition costs about US$7m: in-flight entertainment US$1.5m, business seats US$1.5m at US$30,000 each, economy seats US$1m at US$5,000 each, repaint US$100,000, engineering US$100,000 [S]. For a narrowbody a clean redelivery costs US$1-3m and a widebody US$4-10m, more when engines miss the condition. ESTIMATE. Late return means holdover rent, commonly at a premium [R].

Disputes centre on records gaps, engine condition found at borescope, repair documentation, and who pays for airworthiness directives falling due near the return date. The cheapest fix in 2023-26 has been not to return: extension rates near 80% [S].

---

## 4. Maintenance cost

### 4.1 Industry totals (IATA MCX)

| Measure | FY2023 | FY2024 |
|---|---|---|
| Airlines / aircraft in sample | 27 / 2,241 | 28 / 2,703 |
| Maintenance cost per flight hour | US$1,499 | US$1,522 |
| Per flight cycle | US$3,445 | US$3,758 |
| Per aircraft per year | US$4.59m | US$5.05m |
| Average fleet age | 10.6 | 10.6 |
| Utilisation, hours a day | 8.38 | 9.06 |
| Dispatch reliability | 98.75% | 98.86% |

All [S]. FY2024 total spend for the sample was US$13.19bn including overhead [S]. The pre-COVID figure was about US$1,100 per flight hour [R].

World MRO spend (Oliver Wyman via IATA) [S]:

| | 2019 | 2020 | 2021 | 2022 | 2023 | 2024 | 2034 forecast |
|---|---|---|---|---|---|---|---|
| US$bn | 82 | 50 | 62 | 79 | 94 | 104 | 124 |
| Share of airline expenses | 10% | 10% | 11% | 11% | 11.1% | 11.5% | n/a |

By segment, 2019 to 2024 [S]: engines 41% to 50%, components 17% to 18%, airframe heavy maintenance 26% to 17%, line maintenance the remaining 15-16%. Widebodies are 20.3% of the active fleet and 42% of spend [S].

Derived from those figures (ESTIMATE by arithmetic): US$104bn over 28,221 active aircraft is US$3.7m an aircraft; widebodies average about US$7.6m a year each and everything else about US$2.7m. At typical utilisation that is roughly US$1,750 per flight hour for a widebody, US$1,000 for a narrowbody and US$750 for regional aircraft. The DMC column of the master table is scaled to these.

US check: Airlines for America puts maintenance at US$18.35 per block minute in 2025, US$1,101 per block hour, 18.6% of direct aircraft operating cost [S].

### 4.2 Checks: intervals, cost, downtime

Intervals (Aviation Week 2018 via Wikipedia, plus IBA 2025) [S]:

| Type | A-check | C-check | Structural / heavy |
|---|---|---|---|
| A320 family | 750 FH, 750 FC or 4 months | 12,000 FH, 8,000 FC or 36 months | 6 and 12 years |
| 737NG | 150 / 600 FH | 7,500 FH or 730 days | at 8-12 years |
| A220 | 1,000 FH | 8,500 FH | n/a |
| E-Jet / E2 | 1,000 FH | 7,500 FH / 10,000 FH | n/a |
| CRJ700 series, Dash 8 | 800 FH | 8,000 FH | n/a |
| ATR 42/72 | 750 FH | 5,000 FH | 2, 4, 8 years |
| 767-300ER | 750 FH or 300 FC | 6,000 FH, 3,000 FC or 18 months | n/a |
| 747-400 / 747-8 | 600 / 1,000 FH | 7,500 / 10,000 FH | 6 years systems; 8, 8, 6 years structures |
| A330 | n/a | 24-36 months | four checks at 6 years, two at 12 years |
| 777 | n/a | n/a | three checks at 8 years |
| 787, A350 | n/a | every 2-3 years | two checks at 12 years; landing gear at 12 years |

Labour and downtime [S]: an A-check takes 50-70 man-hours and at least 10 hours, usually overnight. A C-check takes up to 6,000 man-hours and one to four weeks; Boeing and Qantas put it at one to three weeks. A D-check takes up to 50,000 man-hours; three to six weeks on a current type, months on an old 747. Aircraft get two or three D-checks in a life.

737NG base checks in detail (Aircraft Commerce 2013) [S]: on a 24-month interval, routine labour runs from 600-700 man-hours at the first C-check to 1,700 at the fourth and about 3,250 at the sixth, which is the heavy one. Defect rectification adds 125 man-hours at the first check and 2,300-2,650 at the sixth. With access, cabin, modifications and component changes the six checks of the first 12-year cycle total 21,000-22,000 man-hours, about 0.6-0.7 per flight hour, plus US$360,000 of materials and US$150,000-175,000 of cabin parts. The second cycle needs 10% more routine labour and far more rectification (3,000-3,500 man-hours at the twelfth check).

Heavy check cost [S]: Aircraft Value News, 2018, found actual D-check cost ran 50-70% above the manufacturer's estimate:

| Type | Estimated (US$m) | Actual (US$m) |
|---|---|---|
| 737-800 | 0.65 | 1.0 |
| 777-200ER | 2.5 | 4.0 |
| 777-300ER | 2.7 | 4.5 |
| 747-400 | 4.0 | 6.0 |
| ATR 72 | n/a | 0.5 |

ATR 72 C-check labour runs from 590 man-hours at the first to 3,685 at the eighth [S]. World spend on C- and D-checks in 2022 was about US$4.9bn: US$3.0bn labour and US$1.9bn material across 44 million man-hours, 57% of them on single-aisle aircraft [S]. Labour is up to 70% of a heavy check's cost [S].

Airbus claims the A350 airframe costs 25% less to maintain than the A330, and Boeing claims up to 30% for the 787-8 against older types [S]. The mechanism is fewer heavy checks: two in 25 years against three for a 777 and six for an A330 [S].

### 4.3 Engines

| Engine | Shop visit cost | Interval and life | Notes |
|---|---|---|---|
| CFM56-7B / -5B | 2008: US$1.2-1.3m light core, US$1.5-1.6m heavier core [S]. 2025-26 broker ranges: US$1.8-2.5m performance restoration, over US$3.5m with full parts replacement [S]. Trade press: US$3-6m or more [R] | First run over 20,000 hours; fleet average about 30,000 hours to first visit, record 50,000 [S]. Mature runs 10,000-15,000 hours [R] | Parts lives: hot section 20,000 cycles, compressor 25,000, fan and booster 30,000; full set over US$3m at an old price [S], US$4-4.6m now [R]. About 2,500 shop visits a year at the 2025 peak [S] |
| V2500-A5 | 2008: US$1.25m to US$3.5m by workscope [S]. 2013: US$2.25-3.63m [S] | All life-limited parts 20,000 cycles [S]; removal intervals in the 2013 table run from 8,500-9,500 cycles down to 5,000-5,500 on later runs (table garbled in extraction) [S] | High-thrust variants cost about 15% more per visit [S]. Engine shop labour US$85-100 a man-hour in 2013 [S] |
| LEAP-1A / -1B | US$2.0-4.5m+, 60-120+ days (broker) [S] | Neutral climate: about 17,000 cycles or 8 years to first visit. Hot and dusty: 1.5 years before the turbine blade fix, about 4 years or 8,000 cycles after [S] | The -1B durability kit was due for certification in H1 2026 [S]. One wet-lease operator reports 30% shorter time on wing in harsh conditions [S] |
| PW1100G (GTF) | US$2.5-5.0m+, 70-140+ days (broker) [S] | Gearbox designed for 30,000 hours; parts lives up to 25,000 cycles [S] | See the recall below |
| GE90-115B | US$6-9m (broker) [S]; US$10-15m with parts [R] | 20,000-25,000 hours first run (broker gives cycles) [S] | Scarce shop slots raised used GE90 values about 50% [S, headline only] |
| Trent 700 | US$3-5m (broker) [S] | n/a | Mostly on Rolls-Royce flight-hour contracts |
| Trent 1000 | n/a | The XE standard claims three times the time on wing of early builds [S] | Shop visits rose from 430 in 2024 to 517 in 2025 for the retrofit [S] |
| GEnx, Trent XWB | n/a | n/a | Turnaround up to about 200 days in 2026 [S] |
| PW127 (ATR) | US$0.3-1.0m [S] | n/a | The PW127XT claims 20% lower maintenance cost [S] |

Cost structure of a shop visit: material 60-70%, labour 20-30%, sub-contracted repairs 10-20% [S]. Engines are now half of all maintenance spend [S].

**Pratt & Whitney powder-metal recall.** Contaminated powder metal made at RTX's Clayville, New York plant between Q4 2015 and Q3 2021 [S]. In July 2023 Pratt ordered 1,200 of about 3,000 PW1100G engines inspected; in September 2023 it extended the recall to all 3,000 and said 600-700 extra engines would come off wing through 2026 [S]. Each inspection visit takes 250-300 days; a good shop should manage 120 [S]. Pratt forecast an average of 350 aircraft parked through 2026 with a peak near 650 in early 2024 [S]. In fact the peak came later: 648 GTF-powered aircraft (A320neo, A220, E2) were stored in March 2025, 28% of a fleet of 2,344 [S]. RTX took a US$3bn pre-tax charge in Q3 2023; the gross cost to Pratt and its partners is US$6-7bn, about 80% of it customer compensation [S]. Of aircraft under 10 years old in storage (over 1,100, 3.8% of the world fleet against 1.3% in 2015-18), 69% have PW1000G engines [S]. Wizz Air had 41 aircraft grounded at 30 September 2024 and expected 40-45 at a time for 18 months, with the last back in late 2027 or early 2028 [S].

**Earlier GTF trouble** [S]: IndiGo replaced 69 engines between mid-2016 and early 2018 and was compensated; India's regulator ordered all 98 of its A320neos re-engined by 2020. In 2018 Airbus stopped accepting engines and over 100 finished aircraft sat without them.

**Trent 1000** [S]: turbine blade cracking found at ANA in early 2016. Up to 44 787s were grounded at the peak. Cost to Rolls-Royce over 2017-23: £2.4bn.

### 4.4 Cost against age

The standard three phases [R]:

- **Honeymoon, years 0-5.** Warranty covers defects, structural checks have not come due, engines are on their first run. Cost is 30-40% below the mature level.
- **Mature, years 6-12.** First engine shop visits, first 6-year check, landing gear at 10 years.
- **Ageing, year 13 on.** Rectification work grows about 3% a year; second-run engines stay on wing half as long.

Sourced support: 737NG routine labour is 10% higher in the second check cycle and rectification labour grows from about 4,600 to 5,600-6,000 man-hours [S]; A320 base check cost rises from US$168 to US$185 per flight hour between cycles [S]; ATR base checks from US$50 to US$80 [S]. SMBC notes more unplanned work as the world fleet ages [S].

The honeymoon failed for the newest engines. LEAP and GTF time on wing fell well below expectation [S], and an Aviation Week headline quotes an executive saying low reliability is "killing" the fuel saving [S, headline only].

### 4.5 Reliability and aircraft on ground

IATA sample average dispatch reliability 98.86% [S]. Claims by type: A220 99.9% at Swiss in 2017; GTF-powered A320neo 99.91% per Pratt and 99.6% per Airbus, with Lufthansa below its 99.8% target [S]. Mature narrowbodies run 99.5-99.8% and new widebodies 99.2-99.6% [R].

Boeing's old estimate: a one to two hour grounding costs US$10,000-20,000, and the cost can reach US$150,000 an hour depending on type and route [S, single dated source].

### 4.6 Labour rates and outsourcing

- Aircraft Commerce used US$50 a man-hour for airframe checks in 2010-13 and quoted a world average of US$60 for base maintenance in 2011; engine shops US$85-100 [S].
- Labour rate inflation is 5.5-6.0% a year against about 3% before the pandemic; engine labour rose 7.4% in Europe and 6.4% in North America in the latest survey [S]. Airline salaries rose 8% in 2024 and are 26% above 2019 [S].
- Asia-Pacific does 50-70% of the world's heavy checks [S].
- Current airframe rates by region, ESTIMATE from recalled quotes: North America US$75-110, Western Europe US$75-100, Eastern Europe and Turkey US$50-65, China and South-East Asia US$45-60, Central America (Aeroman in El Salvador, Coopesa in Costa Rica) US$40-55.

On a 6,000 man-hour check a US$40 rate gap is US$240,000, set against two ferry flights and extra days out of service. That is why US narrowbody checks go to Central America and widebody checks to Asia.

---

## 5. The whole operating cost picture

### 5.1 Cost per block hour, United States

Airlines for America, 2025, from Form 41 data for US scheduled passenger airlines [S]:

| Component | Per block minute | Per block hour | Share |
|---|---|---|---|
| Crew (pilots and flight attendants) | US$37.01 | US$2,221 | 37.6% |
| Fuel | US$29.34 | US$1,760 | 29.8% |
| Maintenance | US$18.35 | US$1,101 | 18.6% |
| Aircraft ownership | US$9.76 | US$586 | 9.9% |
| Other | US$3.95 | US$237 | 4.0% |
| **Total direct aircraft operating cost** | **US$98.41** | **US$5,905** | |

The FAA's "Economic Values for Investment and Regulatory Decisions" (2024 update, data to June 2023) has the same cut by carrier group in Table 4-2 [S]. I could not open it. Pull it by hand before finalising: it is the best public source of block-hour cost by aircraft class.

By class, scaled around the A4A average. ESTIMATE:

| Class | Direct operating cost per block hour |
|---|---|
| 70-seat turboprop | US$2,000-2,400 |
| 76-seat regional jet | US$2,800-3,400 |
| 150-180 seat narrowbody | US$4,800-5,800 |
| 200-230 seat narrowbody | US$5,500-6,500 |
| 250-300 seat widebody | US$9,000-11,500 |
| 350+ seat widebody | US$12,500-16,000 |

### 5.2 Unit cost by carrier

US cost per available seat mile excluding fuel and transport-related expense, cents (MIT Airline Data Project, Form 41) [S]:

| Carrier | 2010 | 2015 | 2018 | 2019 |
|---|---|---|---|---|
| American | 9.21 | 8.66 | 10.75 | 11.14 |
| Delta | 8.06 | 9.73 | 10.21 | 10.40 |
| United | 8.41 | 9.37 | 10.08 | 10.15 |
| Network average | 8.49 | 9.53 | 10.35 | 10.56 |
| Southwest | 7.76 | 8.67 | 8.90 | 9.67 |
| JetBlue | 6.67 | 7.92 | 8.56 | 8.63 |
| Alaska | 7.96 | 7.55 | 8.49 | 8.13 |
| Hawaiian | 8.83 | 8.23 | 9.33 | 9.37 |
| Allegiant | 5.06 | 5.68 | 6.41 | 6.20 |
| Frontier | 6.99 | 6.34 | 6.04 | 5.65 |
| Spirit | 5.90 | 5.66 | 5.75 | 5.71 |
| Ultra-low-cost average | 6.18 | 5.89 | 5.97 | 5.78 |

2020 is unusable (network average 17.68). Since 2019 labour contracts have pushed network carriers to roughly 12-14 cents and ultra-low-cost carriers to 7-8 [R].

Europe: Ryanair averaged €70 revenue and €62 cost per ticket in 2025 [S], about €36 excluding fuel [R]. Wizz Air runs about 2.5-3 euro cents per seat-kilometre excluding fuel, and easyJet about £45-50 per seat excluding fuel [R].

### 5.3 World cost shares

World airline revenue in 2024 was US$966bn and expenses US$904bn; jet fuel averaged US$99 a barrel [S]. Fuel was 30% of operating cost in 2024 [S] and maintenance 11.5% [S]. The rest, as a model default [R]: labour 25-28%, aircraft ownership (depreciation, rent, interest) 10-13%, airport and navigation charges 8-10%, sales and distribution 3-5%, ground handling, catering and passenger service 6-8%, other 8-10%.

---

## 6. Events that reveal contract structure

**GPA, 1992** [S]. Guinness Peat Aviation was the largest lessor of the 1980s, valued at US$4bn, earning US$265m in the year to March 1992, and in 1990 ordered US$20bn of aircraft, about 10% of world production. Its share offering in 1992 failed in the post-Gulf War slump. With about US$10bn of debt it sold aircraft and management to GE Capital in 1993, which became GECAS. The remainder securitised 229 aircraft for US$4bn in 1996 and became AerCap in 2005. The lesson is speculative ordering financed short.

**ILFC to AerCap** [S]. AerCap bought ILFC from AIG in May 2014 for US$3.0bn cash and US$4.6bn in stock, gaining over 1,300 aircraft, then GECAS in November 2021 for about US$23bn cash, US$1bn in notes and 111.5 million shares.

**Kingfisher, 2012.** Half the fleet was grounded by early 2012; India's regulator suspended the licence on 20 October 2012 [S]. Lessors spent months trying to deregister aircraft against airport and tax liens; India had acceded to the Cape Town Convention but had no implementing law [R].

**Jet Airways, 2019.** Nearly a quarter of the fleet was grounded for unpaid rent in March 2019; all flying stopped on 17 April 2019 with about US$1.2bn of debt; liquidation ordered 7 November 2024 [S]. Lessors deregistered over 100 aircraft within weeks under 2015 rules that honour deregistration powers of attorney [R].

**Go First, 2023** [S]. Filed for insolvency on 2 May 2023 with 26 aircraft flying and 28 grounded for want of Pratt engines, on dues of ₹11,463 crore (US$1.2bn). The insolvency moratorium blocked repossession for a year. The Delhi High Court ordered deregistration on 26 April 2024 and all 54 leased aircraft were deregistered on 1 May 2024. India exempted aircraft from the moratorium in October 2023 and passed a Cape Town implementing act in 2025 [R].

**Section 1110 and the 2020 Chapter 11s** [R]. Avianca (10 May 2020), LATAM (26 May) and Aeromexico (30 June) used US bankruptcy to reject leases on aircraft they no longer wanted and to renegotiate the rest to hourly rent. Under Section 1110 the airline has 60 days to cure and commit, or return the aircraft.

**COVID rent deferrals, 2020** [R]. Most lessees asked for relief. The usual deal deferred three to six months of rent, repaid over 12-24 months, often with a lease extension. Cash collection at the big lessors fell to roughly 80-90% for several quarters.

**Russia, 2022.** Sanctions required leases to end by 28 March 2022; Russia re-registered the aircraft and kept them. About 400 of over 500 foreign-leased aircraft, worth roughly US$10bn, stayed [R]. AerCap had 152 aircraft worth US$2.5bn in Russia and Ukraine; 113 were seized. It wrote off US$3.16bn (US$2.66bn net) and reported a US$2bn loss for the quarter [S]. Russian insurers then bought aircraft out with state money: Aeroflot paid AerCap US$645m for 17 aircraft in September 2023, and about US$572m of further settlements followed in December 2023 [S]. AerCap's claim against its own insurers was about US$3.5bn [R]. In June 2025 London's High Court ruled it could recover more than US$1bn under war-risk cover for almost 150 jets [S]. Other write-offs [R]: SMBC Aviation Capital about US$1.6bn, Air Lease about US$800m, BOC Aviation about US$800m, Avolon about US$300m.

**737 MAX grounding, 13 March 2019 to 18 November 2020** [S]. Direct cost to Boeing about US$20bn. A US$4.9bn after-tax charge in Q2 2019 for customer concessions. The January 2021 US settlement included US$1.77bn for airline customers. Southwest reported a US$435m hit in the first nine months of 2019 and shared US$125m of its settlement with staff; TUI put its 2020 cost at €220-245m. Ryanair took cheaper aircraft instead of cash. Lessor Timaero sued for at least US$185m. United bought 19 used 737-700s to fill the gap.

**787 delivery halt** [S]. Boeing delivered 14 787s between November 2020 and August 2022; deliveries resumed 10 August 2022. Wikipedia gives US$6.3bn as the cost of the halt. My recollection is that this is US$2.8bn of abnormal production cost plus a US$3.5bn charge for customer compensation taken in Q4 2021, so do not add the two [R].

**GTF compensation.** About 80% of the US$6-7bn recall cost is customer compensation [S], paid mostly as monthly credits per grounded aircraft. Spirit's 2024 agreement was worth US$150-200m of liquidity [R]. The credits do not cover lost profit; Wizz cut capacity by at least 10% in 2023 [S] and IndiGo wet-leased replacement aircraft [S].

**Shortage, 2023-26** [S]. Deliveries in 2024 were 30% below peak. The backlog is 17,000 aircraft, nearly half the fleet and 13.5 years of production. Average wait for a new aircraft is 5.3 years. About 5,000 aircraft scheduled for 2019-25 were never built. Normal supply is not expected before 2031-34. Airbus has outsold Boeing 2.3 to 1 on the neo against the MAX since 2016. Retirements are near record lows and the A320's average retirement age is 26.

**Fleet tactics.**

- **Ryanair** orders when Boeing is weak [S]: 155 737-800s in January 2002 after the 11 September attacks at "a substantial discount"; 175 in March 2013 (US$15.6bn at list); 75 more MAX 200s in December 2020 at a third of list during the grounding; 300 MAX 10s in May 2023 (€36.3bn at list), at a smaller discount after an 18-month public argument over price.
- **Delta** buys old aircraft cheaply and maintains them in-house. It took all 88 of AirTran's 717s on sublease from Southwest from 2013, and used MD-90s, to retire DC-9s for less than new jets cost [S]. About 90% of its mainline fleet is owned [S].
- **Allegiant** built its business on MD-80s bought for a few million dollars each and flown few hours a day, then moved to used A320s [R].

**Lease against own, by carrier type.** Low-cost carriers using sale and leaseback are 60-100% leased (Wizz, IndiGo, Spirit with 83 leased aircraft at end-2025 [S]). US majors are 10-35% leased (Delta about 10% [S]). Ryanair and Southwest own nearly everything. The world average is about 50% [S].

---

## 7. Other leased and rented equipment

### 7.1 Airport gates and terminals, United States

Airline payments to an airport divided by enplaned passengers (FAA Form 5100-127, fiscal 2025) [S]: Atlanta US$4.48, the lowest of the 30 large hubs; New York JFK US$38.37, the highest; large-hub median US$14.83. Los Angeles projects US$38.10 for fiscal 2026 [S]. Roughly, Newark and LaGuardia sit in the US$30s, Chicago O'Hare and San Francisco in the US$20s, and Charlotte near Atlanta's level [R]; the source page did not give these.

Rate-setting methods [S]:

| Method | Who carries revenue risk | Examples |
|---|---|---|
| Residual | Airlines pay whatever non-airline revenue does not cover | Chicago O'Hare and Midway, Miami, San Francisco, Detroit |
| Hybrid residual | Shared; airlines backstop debt coverage | Denver, Dallas/Fort Worth, Los Angeles, Seattle |
| Hybrid compensatory | Mostly the airport | Atlanta, Minneapolis, Philadelphia, Tampa |
| Compensatory | Airport; airlines pay a cost share for space used | JFK, Newark, LaGuardia, Boston, Phoenix |

Carriers that sign the use-and-lease agreement pay signatory rates. Non-signatories pay a premium, most often 25%, in a range of 5-30% [S]. Signatories holding a majority of traffic get a veto over capital projects [S].

Gates [S]: exclusive use is being phased out. Preferential use, the norm at large hubs, gives scheduling priority subject to a minimum use rule; Philadelphia requires five turns or 625 seats a day. Common-use gates are charged per turn.

Prices [S]:

- Seattle, per turn on a common-use gate: US$1,379 widebody, US$690 narrowbody, US$345 regional; non-signatories 25% more.
- Seattle, preferential gate: US$1,617,970 a year, plus US$90,222 for the loading bridge.
- Miami, preferential gate: US$789,089 a year.
- San Francisco terminal space: US$512 per square foot a year for holdrooms and ticket counters, down to US$51 for the lowest category.
- Houston: US$176 per flight for holdroom, apron and bridge together.
- Landing fees in the source's worked examples: US$3.6-7.4 per 1,000 lb of landed weight.

Outside the US, charges are per passenger and per tonne under published tariffs, with no gate leases [R].

### 7.2 Ground equipment, training, IT, component pools

None of this could be searched; all [R] or ESTIMATE.

- **Ground support equipment.** Most airlines outsource handling away from hubs. Narrowbody turn: US$400-900 below-wing in the US, €800-2,500 full handling in Europe. Equipment prices if owned: narrowbody pushback tug US$100,000-250,000, widebody towbarless tractor US$600,000-1m, ground power unit US$50,000-90,000, belt loader US$40,000-80,000, de-icing truck US$300,000-600,000. Leasing costs 1.5-2.5% of value a month.
- **Simulators and training.** A full-flight simulator costs US$10-15m for a narrowbody and US$15-20m for a widebody. Bought-in time: US$350-600 an hour for a narrowbody, US$600-1,000 for a widebody. A type rating costs an airline US$25,000-40,000 a pilot. Recurrent training needs about 16 simulator hours a pilot a year.
- **Passenger service system.** Charged per passenger boarded: about US$0.50-1.50 for reservations, inventory and departure control (Amadeus Altéa, Sabre), with low-cost platforms (Navitaire) lower. Global distribution system bookings cost several dollars a segment on top.
- **Component pooling.** A flight-hour contract covering pool access, repair and logistics for rotables costs US$180-300 per flight hour for an A320 or 737, US$120-220 for regional aircraft, US$350-600 for a widebody. The 2006 A320 figure was US$180 for rotables plus US$120 for heavy components [S]. Providers: AFI KLM E&M, Lufthansa Technik, AJW, AAR. Airlines in the IATA sample each hold spares inventory; the public report does not give the figure.

---

## Model recommendations

### A. Lease contract object

| Field | Dry operating lease | Hourly-rent variant | ACMI variant |
|---|---|---|---|
| Rent | Fixed monthly, in advance: current market value x lease rate factor, locked at signing | Rate per flight hour x hours flown, with a floor of 30-50% of the fixed rent | Rate per block hour x max(hours flown, guaranteed hours) |
| Term | New narrowbody 8-12 years (default 10); new widebody 12; used 3-8; extension 2-6 | 6-24 months, then reverts to fixed; lessor asks 1-3 extra years of term | 1-24 months (default one summer season, 5 months) |
| Deposit | 2 months' rent (0 for top credits, 3 for weak) | unchanged | 1 month of guaranteed hours |
| Maintenance | Lessee pays everything. Weak credit: cash reserves monthly. Strong credit: no reserves, pays end-of-lease compensation | unchanged | Lessor pays; included in the rate |
| Crew, insurance | Lessee | Lessee | Lessor (lessee supplies cabin crew on a damp lease, 10-15% lower rate) |
| Fuel, airport, navigation, handling | Lessee | Lessee | Lessee |
| Return | Fresh C-check; engines and parts with minimum life; cash true-up to half-life or to delivery condition | unchanged | Aircraft simply goes back |
| Early termination | No right. Charge: rent to the earlier of lease end or 12 months, plus the return true-up. Lessor waives part in a tight market (it can re-lease at a higher rent) and waives nothing in a glut | n/a | Guaranteed hours for the remaining term, usually capped at 2-3 months |
| Default | Lessor repossesses after 60 days (US rule). Jurisdiction risk: add 3-12 months and a rent premium where repossession is hard (India before 2025, Russia after 2022) | unchanged | Lessor withdraws aircraft immediately |

All defaults are ESTIMATE calibrated to the sections above.

### B. Default numbers

**Value.** `value(type, age) = new_value(type) x age_factor(age) x cycle_value(t)`

| Age (years) | 0 | 5 | 10 | 15 | 20 | 25 |
|---|---|---|---|---|---|---|
| Narrowbody and regional | 1.00 | 0.74 | 0.52 | 0.33 | 0.19 | 0.10 |
| Twin-aisle | 1.00 | 0.68 | 0.47 | 0.28 | 0.14 | 0.07 |
| Four-engine | 1.00 | 0.60 | 0.38 | 0.18 | 0.08 | 0.04 |

Floor the value at engine and parts value: about US$4-6m for a narrowbody with run-out engines, more when engine shop capacity is short.

**Lease rate factor, monthly.** `lrf(age) = 0.0070 + 0.00020 x age` up to age 10, then `+ 0.00055` a year beyond. That gives 0.70% new, 0.80% at 5 years, 0.90% at 10, 1.18% at 15, 1.45% at 20. Add 0.03-0.10 percentage points for a weak credit and subtract 0.03-0.05 for a top credit or a 12-year term.

**Purchase price.** List x (1 - discount): 45% for an order under 20 aircraft, 55% for 20-99, 62% for 100 or more, plus 5 points when the manufacturer is in trouble (2002, 2020). Turboprops and regional jets 20-35%. Sale and leaseback pays the airline the market value on delivery day.

**Reserves, whole aircraft, when paid in cash:** from the master table. As a shortcut, 40-45% of fixed rent at 300 flight hours a month for a narrowbody.

**Accounting.** Show rent as cash cost; capitalise at 7 times annual rent for leverage tests.

### C. Maintenance cost against age

`maintenance per flight hour = mature_rate(type) x age_multiplier(age) x labour_region x inflation(t)`

| Age | 0-2 | 3-5 | 6-7 | 8-12 | 13-16 | 17-20 | 21-25 |
|---|---|---|---|---|---|---|---|
| Multiplier | 0.60 | 0.75 | 0.90 | 1.00 | 1.12 | 1.25 | 1.40 |

- `mature_rate` from the DMC column of the master table. Split it 45% engines, 20% components, 20% line, 15% airframe checks for a narrowbody; 50 / 18 / 15 / 17 for a widebody.
- Smooth this rate for the profit and loss account, but take the cash and the downtime at the events in D.
- Inflate 3% a year in normal times and 6% in 2022-26.
- Labour region scales only the airframe-check share: 1.0 for North America and Western Europe, 0.65 for Eastern Europe and Turkey, 0.55 for Asia and Central America, with two ferry flights and two extra days out of service.
- New engine types: for the first eight years after a type enters service, draw a random early-removal rate. Calibrate to the GTF: up to 28% of the fleet parked at the worst point, with manufacturer credits covering roughly the rent on a grounded aircraft and none of the lost margin.

### D. Check and shop-visit calendar

Days out of service. ESTIMATE, consistent with section 4.2.

| Event | Narrowbody | Twin-aisle | Regional jet / turboprop |
|---|---|---|---|
| A-check | every 4 months; overnight, 0 revenue days if scheduled | every 2-3 months; 1 day | every 3-4 months; overnight |
| C-check | every 24-36 months; 7 days; US$0.3-0.5m | every 24-36 months; 12 days; US$0.7-1.2m | every 30-48 months; 8 days; US$0.2-0.4m |
| Intermediate structural | 6 years; 25 days; US$1.0m | 6 or 8 years; 35 days; US$2.5-4.5m | 8 years; 20 days; US$0.6-0.9m |
| Heavy structural | 12 years; 40 days; US$2.0m | 12 years; 45 days; US$3.5-4.5m (US$10m+ four-engine) | 12 years; 25 days; US$0.9m |
| Landing gear | 10 years (12 on 787, A350); in a C-check; US$0.35-0.5m | US$1.0-1.6m | US$0.2-0.3m |
| Engine shop visit | first at 20,000-25,000 hours on a mature type, then every 10,000-15,000; US$3-6m. New-generation: first at 8,000-17,000 cycles by climate | first at 18,000-25,000 hours, then 12,000-18,000; US$6-13m | 8,000-14,000 hours; US$0.5-2.0m |
| Engine life-limited parts | at 20,000-30,000 cycles, with a shop visit; US$4-5m a set | at 10,000-20,000 cycles; US$8-14m a set | at 15,000-25,000 cycles; US$1-2.5m |

Engine swap rule: with a spare (owned, leased at the section 2.4 rates, or from a pool) the aircraft loses two days. Without one it is grounded for the shop turnaround: 60-90 days in normal times, 100-150 in 2023-26, 250-300 for a recall-class event. Give each airline a spare ratio (8-10% of installed engines is normal) and make spare lease rent rise with the shortage index in E.

Cabin refit at each heavy check or lease transition: US$1-2m narrowbody, about US$7m twin-aisle.

### E. Lease-rate cycle driver

`rent(type, age, t) = value(type, age, t) x lrf(age) x rate_adj(t) x shortage_adj(t)`

- **Interest rates.** `rate_adj = 1 + 0.06 x (swap10y(t - 12 months) - 2.5)`, with the swap rate in percentage points. Each 100 basis points adds 6% to rent after a year. This is about half pass-through, which fits a new A350-900 whose factor fell from 0.79% to 0.73% while rates rose. ESTIMATE.
- **Shortage.** Build an index from stored share of the fleet and manufacturer delivery shortfall against plan:
  `shortage = (normal_stored_share - stored_share) x 2 + delivery_shortfall x 0.5`, clipped to the range -0.5 to +0.25.
  `shortage_adj = 1 + shortage` for rent. Apply 1.3 times the same index to value, since value moves first and furthest in a shortage.
- **Calibration targets** for a new narrowbody, with 2019 = 1.00: 2015 1.05; 2020-21 0.70; 2022 0.87; 2024 1.09; 2026 1.13. For a 10-year-old narrowbody: 2020-21 0.60; 2024 1.05. For a new twin-aisle: 2020-21 0.67; 2024 0.97; 2026 1.05. For four-engine aircraft: 2020-21 0.35-0.40, with no recovery.
- **Asymmetry.** Existing fixed-rent leases do not reprice. Only new leases and extensions take the current rent. In a glut, lessors grant deferrals (3-6 months, repaid over 12-24) and hourly rent before they repossess, because a repossessed aircraft has nowhere to go. In a shortage, extension probability rises from 55% to 80% and the airline pays the current market rent to keep the aircraft.
- **Type shocks.** A grounding of one type raises rent on its direct substitute by 20-30% within six months (737NG during the MAX grounding).

---

## Sources

Fetched this session:

- IATA, Maintenance Cost data eXchange Executive Report, FY2024 data, public version, January 2026: https://www.iata.org/contentassets/8437020db31a4717b70677d9b06b1a45/fy2024-mcx-report_public.pdf (FY2023 headline figures from the search listing of the FY2023 edition)
- IATA, "The global commercial aircraft fleet: shortages cap growth", August 2025: https://www.iata.org/en/iata-repository/publications/economic-reports/the-global-commercial-aircraft-fleet
- IATA / Emerton, "Single aisle engine MRO" (2025): https://www.iata.org/contentassets/85b59d951fc04c1c83fa2aab47824300/single-aisle-aircraft-engines-mro.pdf
- Aviation Strategy, "Jet values and lease rates" (AVAC data): November 2015 https://aviationstrategy.aero/newsletter/Nov-2015/4/Jet-values-and-lease-rates ; November 2019 https://aviationstrategy.aero/newsletter/Nov-2019/5/Jet-Values-and-Lease-Rates ; October 2020 https://aviationstrategy.aero/newsletter/Oct-2020/4/Jet-Values-and-Lease-Rates
- SMBC Aviation Capital, Plane Insights Q2 2025: https://www.smbc.aero/sites/smbc.aero/files/2025-04/SMBC%20AC%20Plane%20Insights%20Q2%2025.pdf
- Acumen Aviation, "Aircraft Valuation in 2026" (May 2026): https://www.acumen.aero/uploads/marketinsight/844315020260512.pdf ; "Narrowbody Aircraft Valuation Trends" (August 2026): https://www.acumen.aero/uploads/marketinsight/730629320260820.pdf
- IBA September 2024 lease rates as reproduced at https://boltflight.com/aircraft-dry-lease-rates-an-in-depth-2025-analysis-of-commercial-aviation-leasing-costs/
- IBA regional values, AviTrader, 7 April 2025: https://avitrader.com/2025/04/07/iba-regional-travel-surge-fuels-aircraft-demand/
- IBA on A350 and 787 heavy checks, AviTrader, 17 August 2025: https://avitrader.com/2025/08/17/company-interview-with-iba-group-ltd-focusing-specifically-on-heavy-maintenance-requirements-for-airbus-a350-and-boeing-b787-aircraft/
- AirlineGeeks, 31 August 2022 (IBA and Cirium figures): https://airlinegeeks.com/2022/08/31/lease-rates-of-a320neo-737-max-rise-due-to-aircraft-shortage/
- ISTAT appraisals: A330-900 (AVITAS, December 2025) https://www.istat.org/ISTAT-Online-DEV/ArtMID/1208/ArticleID/1631/Appraisal-Airbus-A330-900 ; LEAP-1B (September 2025) https://www.istat.org/ISTAT-Online-DEV/ArtMID/1208/ArticleID/1639/Appraisal-CFM-LEAP-1B
- AerCap fact sheet, Q4 2025: https://www.aercap.com/_assets/_23be2bac275f54a46de3040600309cc4/aercap/db/540/14608/file/Aercap_Factsheet+4Q+2025.pdf
- Airlines for America, US passenger carrier delay costs (2025 block-minute costs): https://airlines.org/dataset/u-s-passenger-carrier-delay-costs/
- MIT Airline Data Project, cost per seat mile excluding fuel: http://web.mit.edu/airlinedata/www/Expenses&Related.html
- Sofema, maintenance reserve worked example: https://sassofia.com/blog/aircraft-leasing-maintenance-reserve-considerations/
- mba Aviation, "Maintenance Matters in Aircraft ABS Deals, Part II": https://www.mba.aero/wp-content/uploads/Maintenance-Matters-in-Aircraft-ABS-Deals-Part-2.pdf
- Aircraft Commerce: APU maintenance (issue 109) https://aircraft-commerce.com/wp-content/uploads/aircraft-commerce-docs/Maintenance/2016/109_MTCE.pdf ; A320 and V2500 (issue 91) https://www.aircraft-commerce.com/wp-content/uploads/aircraft-commerce-docs1/Maintenance/2013/ISSUE91_MTCE_A.pdf ; 737NG base checks (issue 90) https://www.aircraft-commerce.com/wp-content/uploads/aircraft-commerce-docs/Maintenance/2013/ISSUE90_MTCE_A.pdf ; plus the A320 (issue 44), 737NG (issue 70), 777 (issue 60), A330 (issue 30), E-Jet (issue 64), CRJ (issue 66), ATR (issue 49), Dash 8 (issue 63), CFM56-7B (issue 58) and V2500 (issue 56) guides via search listings on aircraft-commerce.com
- ACC Aviation ACMI market report as summarised at https://theflyingengineer.com/acc-aviation-acmi-market-2026-report/ ; https://aerocorner.com/blog/wet-leasing-and-dry-leasing/ ; https://www.oag.com/blog/what-is-wet-leasing
- DWU Consulting: cost per enplanement https://dwuconsulting.com/dwu-ai/cost-per-enplaned-passenger ; rate methods https://dwuconsulting.com/dwu-ai/airline-rate-methodologies ; gates https://dwuconsulting.com/dwu-ai/airport-gate-management ; operating leases https://dwuconsulting.com/dwu-ai/airline-operating-leases-sale-leaseback
- Wikipedia articles: Aircraft lease; Aircraft maintenance checks; Aircraft on ground; Pratt & Whitney PW1000G; Financial impact of the Boeing 737 MAX groundings; Rolls-Royce Trent 1000; CFM International CFM56; Go First; Jet Airways; Kingfisher Airlines; AerCap; Guinness Peat Aviation; Ryanair; Wizz Air; IndiGo; Boeing 717; Boeing 737 MAX; Boeing 737 Next Generation; Boeing 777; Boeing 787 Dreamliner; Airbus A220; Airbus A320neo family; Airbus A350; Airbus A380; ATR 72; User:Marc Lacoste/sandbox/Airliner prices
- Search-result summaries only (not opened): Cirium and FlightGlobal on 2020 lease rate falls; CFM press release on LEAP maturity (November 2025); Safe Fly Aviation engine cost pages (a broker, internally inconsistent); Oliver Wyman MRO survey 2026; Aviation Week on labour rates and CFM56 shop visits; Leeham News on pricing; AirInsight and Airways on the powder-metal recall

Not reachable, worth pulling by hand:

- FAA, "Economic Values for Investment and Regulatory Decisions", Section 4, aircraft operating costs: https://www.faa.gov/regulations_policies/policy_guidance/benefit_cost/econ-value-section-4-op-costs.pdf
- IATA, "Guidance Material and Best Practices for Aircraft Leases"
- IATA MCX full report (members only) for the narrowbody, widebody, regional split
- Lessor annual reports (AerCap 20-F, Air Lease 10-K) for deposit, reserve and term disclosures
- Airfinance Journal and ch-aviation for ACMI rates; FAA CATS for cost per enplanement at every airport
