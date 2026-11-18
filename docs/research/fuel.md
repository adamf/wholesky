# Jet fuel as a business variable for airlines

Research notes for parameterising an airline-industry simulator. Compiled 18 November 2026.

## How to read this file

Every number carries one of four provenance tags.

| Tag | Meaning |
|---|---|
| (source link) | Read from the linked document in this session. |
| OWN CALC | Computed in this session from EIA daily spot series (downloaded via FRED: [DJFUELUSGULF](https://fred.stlouisfed.org/series/DJFUELUSGULF), [DCOILBRENTEU](https://fred.stlouisfed.org/series/DCOILBRENTEU), [DCOILWTICO](https://fred.stlouisfed.org/series/DCOILWTICO), [DDFUELUSGULF](https://fred.stlouisfed.org/series/DDFUELUSGULF), [DDFUELNYH](https://fred.stlouisfed.org/series/DDFUELNYH), [DDFUELLA](https://fred.stlouisfed.org/series/DDFUELLA)), 2 April 1990 to 17 November 2026, or from the EUROCONTROL/EEA aircraft database in section 4. Reproducible. |
| ESTIMATE | My judgement, usually interpolating between sourced numbers. |
| RECALLED, UNVERIFIED | A figure I know from the literature or press but could not re-open in this session. Check before relying on it. |

Limits of this pass. The session's shared web-search budget ran out several times, so some items were verified from primary documents reached by direct URL (EIA, IATA PDFs, SEC filings, EUR-Lex, EASA, EUROCONTROL) and others could not be re-checked. Section 8 lists every gap. Three points in the brief need a qualifier and are handled below: Delta's US$2.3bn hedge loss was booked in 2014, not 2015; July 2008 is the peak monthly average, but the daily US Gulf Coast high of 2008 came on 12 September (Hurricane Ike); and Delta paid US$180m gross for Trainer (US$150m net of a state grant).

Context that matters for any 2026 number: air strikes on Iran at the end of February 2026 closed the Strait of Hormuz to commercial shipping. IATA calls it the largest supply disruption in the history of the oil market ([IATA fuel fact sheet, June 2026](https://www.iata.org/en/iata-repository/pressroom/fact-sheets/fact-sheet-fuel/)). The shock is still running: US Gulf Coast jet was US$3.76/gal on 17 November 2026 with Brent at US$92.94 (OWN CALC from EIA daily data).

## Units and conversions

| Quantity | Value | Source |
|---|---|---|
| Barrel | 42 US gal = 158.987 L | definition |
| Density used by Wikipedia's fuel tables | 6.7 lb/US gal = 0.8 kg/L, i.e. 3.03-3.04 kg/US gal | [Wikipedia, Fuel economy in aircraft](https://en.wikipedia.org/wiki/Fuel_economy_in_aircraft) |
| IATA/Platts price conversion | US$94.97/bbl = 226.11 c/gal = US$749.95/t, i.e. 7.897 bbl/t, 331.7 gal/t, implied density 0.7965 kg/L, 3.015 kg/gal | [Aviation Week datasheet of the IATA monitor, 30 Jan 2026](https://aviationweek.com/system/files/datasheets/jet-fuel-price-index-week-ending-jan-30-2026.pdf) |
| Quick rules | US$/t = US$/bbl x 7.9 = US$/gal x 331.7; US$/bbl = US$/gal x 42 | derived |
| CO2 per tonne of fuel | 3.16 t (ICAO convention, as given in the brief); 3.15 t in the EUROCONTROL/EEA database used for EU ETS inventories | [EEA guidebook annex](https://www.eea.europa.eu/en/analysis/publications/emep-eea-guidebook-2023/part-b-sectoral-guidance-chapters/1-energy/1-a-combustion/1-a-3-a-aviation.3) |
| Jet A-1 density specification | 775-840 kg/m3 at 15 C | RECALLED, UNVERIFIED |

For the simulator: hold fuel in kg, price it in US$/t, and convert at 331.7 gal/t when quoting US$/gal so that prices match the IATA monitor.

---

## 1. Price history of jet kerosene

### 1.1 Key levels

US Gulf Coast kerosene-type jet fuel, spot FOB ([EIA series EER_EPJK_PF4_RGC_DPG](https://www.eia.gov/dnav/pet/hist/LeafHandler.ashx?n=PET&s=EER_EPJK_PF4_RGC_DPG&f=A)). Daily extremes and dates are OWN CALC from the daily series.

| Episode | US$/gal | US$/bbl | US$/t | Date or period | Note |
|---|---|---|---|---|---|
| 1998-99 trough, daily low | 0.282 | 11.8 | 94 | 21 Dec 1998 | Monthly lows: Dec 1998 0.304, Feb 1999 0.309 |
| 1998 annual average | 0.403 | 16.9 | 134 | 1998 | Lowest annual average in the series |
| July 2008 monthly average | 3.886 | 163.2 | 1,289 | Jul 2008 | Highest monthly average of 2008 |
| July 2008 daily high | 4.207 | 176.7 | 1,395 | 3 Jul 2008 | Brent 143.95 that day |
| 2008 daily high (Gulf Coast) | 4.814 | 202.2 | 1,597 | 12 Sep 2008 | Hurricane Ike; Brent only 94.37, crack 107.8 |
| Post-crash trough | 1.111 | 46.7 | 369 | 11 Mar 2009 | Dec 2008 monthly 1.375; Feb 2009 monthly 1.259 |
| 2011-14 plateau, average | 2.971 | 124.8 | 985 | Jan 2011 - Aug 2014 | Daily range 2.49-3.38; annual averages 2.69-3.06 |
| 2016 trough | 0.804 | 33.8 | 267 | 20 Jan 2016 | Jan 2016 monthly 0.930 |
| COVID trough | 0.407 | 17.1 | 135 | 27 Apr 2020 | Apr 2020 monthly 0.606; crack briefly negative (-3.0) |
| 2022 peak, Gulf Coast daily | 5.066 | 212.8 | 1,680 | 28 Apr 2022 | Monthly peak Jun 2022 at 4.120 |
| 2022 peak, New York Harbor | 7.61 | 319.6 | 2,524 | 4 Apr 2022 | EIA figure; Platts assessed 7.5861 the same day. Gulf Coast was 3.811 that day. See 1.4 |
| 2023-25 range | 1.819 - 3.992 | 76.4 - 167.7 | 603 - 1,324 | 7 May 2025 / 26 Jan 2023 | Annual averages 2.699, 2.338, 2.114 |
| 2026 pre-shock | 2.262 | 95.0 | 750 | Feb 2026 monthly | Low of the year 1.821 on 7 Jan |
| 2026 first shock month | 3.697 | 155.3 | 1,226 | Mar 2026 monthly | +63% in one month |
| 2026 peak so far | 4.705 | 197.6 | 1,561 | 15 Sep 2026 | Oct 2026 monthly average 4.48 |
| Latest | 3.763 | 158.0 | 1,248 | 17 Nov 2026 | Brent 92.94, crack 65.1 |

IATA's global average (Platts-based, weighted across regions) for the 2026 shock: about US$96/bbl in November 2025, US$188/bbl average in April 2026, US$158/bbl in May ([IATA media briefing, June 2026](https://www.iata.org/en/iata-repository/pressroom/presentations/media-briefing-jet-fuel-update-agm-2026/)); above US$200/bbl by mid-April ([IATA fuel fact sheet](https://www.iata.org/en/iata-repository/pressroom/fact-sheets/fact-sheet-fuel/)); US$194.65/bbl for the week ending 18 September ([Leeham News, 28 Sep 2026](https://leehamnews.com/2026/09/28/soaring-fuel-prices-dim-end-of-year-outlook-for-major-u-s-airlines/)); US$169.82/bbl in the latest weekly reading on the [IATA monitor page](https://www.iata.org/en/publications/economics/fuel-monitor/) (week not stated in the page text). EIA: Gulf Coast jet averaged US$3.91/gal from March through May 2026, about double the start-of-year price ([EIA Today in Energy, 8 Jun 2026](https://www.eia.gov/todayinenergy/detail.php?id=67764)).

### 1.2 Annual table, 1990-2026

Columns 2-9 are OWN CALC, simple means of daily EIA values (they differ from EIA's published annual averages by under 0.01 US$/gal). Brent is Europe Brent spot FOB. IATA columns are from IATA's industry tables: 2004-2017 from the [mid-2018 table](https://www.iata.org/en/iata-repository/publications/economic-reports/airline-industry-economic-performance---2018-mid-year---table), 2019-2026 from the [June 2026 industry statistics fact sheet](https://www.iata.org/en/iata-repository/pressroom/fact-sheets/industry-statistics/) (2025 is an estimate, 2026 a forecast). The last column is the average price actually paid by US scheduled carriers, [US DOT BTS Form 41](https://www.transtats.bts.gov/fuel.asp).

| Year | USGC jet $/gal | $/bbl | $/t | Brent $/bbl | Crack vs Brent $/bbl | Jet/Brent | Crack vs WTI | Jet daily vol (ann.) | IATA jet $/bbl | IATA fuel bill $bn | IATA fuel % opex | US DOT paid $/gal |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1990* | 0.765 | 32.1 | 254 | 23.8 | 8.3 | 1.35 | 7.6 | 66% | - | - | - | - |
| 1991 | 0.608 | 25.5 | 202 | 20.0 | 5.5 | 1.28 | 4.0 | 56% | - | - | - | - |
| 1992 | 0.571 | 24.0 | 189 | 19.3 | 4.7 | 1.24 | 3.4 | 26% | - | - | - | - |
| 1993 | 0.530 | 22.2 | 176 | 17.2 | 5.0 | 1.29 | 3.8 | 28% | - | - | - | - |
| 1994 | 0.493 | 20.7 | 163 | 15.8 | 4.9 | 1.31 | 3.4 | 46% | - | - | - | - |
| 1995 | 0.488 | 20.5 | 162 | 17.0 | 3.4 | 1.20 | 2.0 | 27% | - | - | - | - |
| 1996 | 0.613 | 25.7 | 203 | 20.9 | 4.9 | 1.23 | 3.8 | 55% | - | - | - | - |
| 1997 | 0.560 | 23.5 | 186 | 19.2 | 4.3 | 1.22 | 3.0 | 35% | - | - | - | - |
| 1998 | 0.404 | 16.9 | 134 | 12.7 | 4.2 | 1.33 | 2.5 | 48% | - | - | - | - |
| 1999 | 0.478 | 20.1 | 159 | 18.1 | 1.9 | 1.11 | 1.2 | 54% | - | - | - | - |
| 2000 | 0.848 | 35.6 | 281 | 28.8 | 6.9 | 1.24 | 5.2 | 70% | - | - | - | 0.80 |
| 2001 | 0.714 | 30.0 | 237 | 24.5 | 5.5 | 1.22 | 3.8 | 52% | - | - | 12-13 (a) | 0.78 |
| 2002 | 0.685 | 28.8 | 227 | 25.1 | 3.6 | 1.14 | 2.4 | 37% | - | - | 12-13 (a) | 0.71 |
| 2003 | 0.821 | 34.5 | 272 | 29.0 | 5.5 | 1.19 | 3.1 | 54% | 34.7 (a) | 44 (b) | 12-13 (a) | 0.84 |
| 2004 | 1.151 | 48.3 | 382 | 38.8 | 9.6 | 1.25 | 5.9 | 55% | 49.7 | 65 | 17.3 | 1.15 |
| 2005 | 1.719 | 72.2 | 570 | 54.9 | 17.3 | 1.32 | 15.8 | 71% | 71.0 | 91 | 22.2 | 1.65 |
| 2006 | 1.922 | 80.7 | 637 | 65.2 | 15.5 | 1.24 | 14.7 | 40% | 81.9 | 127 | 28.4 | 1.95 |
| 2007 | 2.145 | 90.1 | 712 | 72.6 | 17.5 | 1.24 | 17.5 | 27% | 90.0 | 146 | 29.8 | 2.09 |
| 2008 | 2.964 | 124.5 | 983 | 96.9 | 27.6 | 1.28 | 24.8 | 67% | 126.7 | 203 | 35.6 | 3.06 |
| 2009 | 1.664 | 69.9 | 552 | 61.7 | 8.2 | 1.13 | 7.9 | 44% | 71.1 | 134 | 28.3 | 1.89 |
| 2010 | 2.150 | 90.3 | 713 | 79.6 | 10.7 | 1.13 | 10.8 | 27% | 91.4 | 151 | 28.3 | 2.27 |
| 2011 | 2.996 | 125.8 | 994 | 111.3 | 14.6 | 1.13 | 31.9 | 26% | 127.5 | 191 | 30.7 | 3.05 |
| 2012 | 3.056 | 128.4 | 1014 | 111.6 | 16.8 | 1.15 | 34.3 | 20% | 129.6 | 228 | 33.2 | 3.16 |
| 2013 | 2.920 | 122.7 | 969 | 108.6 | 14.1 | 1.13 | 24.7 | 19% | 124.5 | 230 | 33.2 | 3.03 |
| 2014 | 2.694 | 113.1 | 894 | 99.0 | 14.1 | 1.14 | 19.6 | 23% | 114.8 | 224 | 30.6 | 2.85 |
| 2015 | 1.521 | 63.9 | 505 | 52.3 | 11.6 | 1.22 | 15.2 | 42% | 66.7 | 174 | 26.4 | 1.84 |
| 2016 | 1.252 | 52.6 | 415 | 43.6 | 8.9 | 1.20 | 9.3 | 48% | 52.1 | 135 | 20.8 | 1.45 |
| 2017 | 1.558 | 65.4 | 517 | 54.1 | 11.3 | 1.21 | 14.6 | 31% | 66.7 | 149 | 21.4 | 1.69 |
| 2018 | 2.027 | 85.1 | 672 | 71.3 | 13.8 | 1.19 | 19.9 | 24% | - | ~180 (c) | ~24 (c) | 2.15 |
| 2019 | 1.879 | 78.9 | 623 | 64.3 | 14.6 | 1.23 | 21.9 | 26% | 80.0 | 190 | 23.9 | 2.00 |
| 2020 | 1.099 | 46.2 | 364 | 42.0 | 4.2 | 1.10 | 7.0 | 76% | 47.0 | 80 | 16.1 | 1.43 |
| 2021 | 1.858 | 78.0 | 616 | 70.9 | 7.2 | 1.10 | 9.9 | 38% | 78.0 | 106 | 19.0 | 1.98 |
| 2022 | 3.374 | 141.7 | 1119 | 100.9 | 40.8 | 1.40 | 46.8 | 72% | 139.0 | 215 | 29.6 | 3.37 |
| 2023 | 2.699 | 113.4 | 895 | 82.5 | 30.9 | 1.37 | 35.8 | 51% | 112.0 | 269 | 31.2 | 2.85 |
| 2024 | 2.338 | 98.2 | 775 | 80.5 | 17.7 | 1.22 | 21.6 | 31% | 99.0 | 261 | 27.8 | 2.54 |
| 2025 | 2.114 | 88.8 | 701 | 69.1 | 19.6 | 1.28 | 23.4 | 37% | 90.0 | 252 | 25.4 | 2.33 |
| 2026* | 3.539 | 148.6 | 1174 | 96.3 | 52.3 | 1.54 | 61.3 | 68% | 152.0 | 350 | 31.4 | 3.44 |

\* 1990 covers 2 April onward. 2026 is year to date: spot to 17 November, DOT January-September, IATA full-year forecast.
(a) [IATA economic briefing, February 2010](https://www.iata.org/en/iata-repository/publications/economic-reports/airline-fuel-labourcosts): fuel was 12-13% of operating cost in 2001-2003; jet averaged US$34.7/bbl in 2003 and US$126.7 in 2008.
(b) IATA release quoted via search summary: US$44bn in 2003 at US$29 Brent.
(c) The mid-2018 table forecast US$188bn and 24.2% for 2018; the [December 2019 fuel fact sheet](https://www.iata.org/contentassets/25e5377cf53c4e48bbaa49d252f3ab03/fact-sheet-fuel.pdf) says the 2019 bill of US$188bn was 4.7% above 2018, implying about US$180bn. IATA's editions disagree slightly (2019 is US$188bn and 23.7% in the 2019 sheet, US$190bn and 23.9% in the 2026 sheet).
Crack versus WTI in 2011-2014 is inflated by the Brent-WTI dislocation; use Brent as the reference crude.

Monthly averages (US$/gal) for the four shock windows, OWN CALC:

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1990 | - | - | - | 0.540 | 0.515 | 0.494 | 0.535 | 0.791 | 1.012 | 1.196 | 0.971 | 0.803 |
| 2008 | 2.605 | 2.728 | 3.124 | 3.365 | 3.738 | 3.878 | 3.886 | 3.271 | 3.375 | 2.315 | 1.880 | 1.375 |
| 2020 | 1.770 | 1.509 | 0.953 | 0.606 | 0.686 | 0.983 | 1.084 | 1.112 | 1.006 | 1.047 | 1.133 | 1.324 |
| 2022 | 2.446 | 2.682 | 3.495 | 3.914 | 3.902 | 4.120 | 3.478 | 3.339 | 3.262 | 3.715 | 3.160 | 2.897 |
| 2026 | 2.031 | 2.262 | 3.697 | 3.928 | 3.943 | 3.042 | 3.403 | 3.724 | 4.39 | 4.48 | 4.01 (part) | - |

The full monthly series since April 1990 is [MJFUELUSGULF](https://fred.stlouisfed.org/series/MJFUELUSGULF); it loads as CSV with no key.

### 1.3 Crack spread (jet minus crude)

| Measure | Value | Source |
|---|---|---|
| USGC jet minus Brent, 1990-2003 | median 4.8, 5th-95th percentile 2.5-8.6 US$/bbl; jet/Brent median 1.24 | OWN CALC |
| 2004-2019 | median 13.2, 5th-95th 6.8-24.9; jet/Brent median 1.18 | OWN CALC |
| 2010-2019 | median 12.7, 5th-95th 7.8-18.9; jet/Brent median 1.17 (5th-95th 1.10-1.28) | OWN CALC |
| 2021 to Nov 2026 | median 20.8, 5th-95th 4.7-66.6, mean 27.5; jet/Brent median 1.28, 95th percentile 1.69 | OWN CALC |
| 2020-21 compression | annual averages 4.2 and 7.2; daily minimum -3.0 | OWN CALC |
| 2022 | annual average 40.8; monthly averages 60 (Apr), 50 (May, Jun), 63 (Oct); daily maximum 107.0 on 28 Apr 2022 | OWN CALC |
| 2008 | annual average 27.6; daily maximum 107.8 on 12 Sep 2008 (hurricane) | OWN CALC |
| 2005 | daily maximum 72.1 on 5 Oct 2005 (Katrina and Rita) | OWN CALC |
| 2026 to date | average 52.3; monthly 52 (Mar), 47, 58, 42, 58, 65, 70 (Sep), 64, 73 (Nov part); daily maximum 80.5 on 30 Sep | OWN CALC |
| IATA global crack, post-COVID vs pre-COVID | 2022-present average is over US$12/bbl higher than 2012-19 | [IATA chart of the week, 11 Sep 2026](https://www.iata.org/en/iata-repository/publications/economic-reports/structural-shifts-increase-jet-fuel-crack-risks/) |
| IATA: 2025 and 2026 | about 20-30 US$/bbl in 2025; 40-70 during the 2026 crisis; 2026 average forecast above 57, a record | [IATA briefing](https://www.iata.org/en/iata-repository/pressroom/presentations/media-briefing-jet-fuel-update-agm-2026/), [fact sheet](https://www.iata.org/en/iata-repository/pressroom/fact-sheets/fact-sheet-fuel/) |
| IATA weekly readings 2026 | 24.57 (30 Jan), about 80 record in April, 70.92 (21 Aug), 67.14 (18 Sep) | [Aviation Week datasheet](https://aviationweek.com/system/files/datasheets/jet-fuel-price-index-week-ending-jan-30-2026.pdf), [Aviation Today](https://www.aviationtoday.com/2026/06/18/iran-cease-fire-raises-hopes-but-iatas-june-outlook-flags-lingering-turbulence/), [Gulf Times](https://www.gulf-times.com/article/731943/business/middle-east-maintains-lowest-global-jet-fuel-price-despite-80-annual-rise-iata), [Leeham](https://leehamnews.com/2026/09/28/soaring-fuel-prices-dim-end-of-year-outlook-for-major-u-s-airlines/) |
| North West Europe jet crack, 2026 | all-time high above 121 US$/bbl in March 2026 against about 30 before the war (LSEG data); 75.38 on 21 Aug 2026 | [Reuters via Yahoo, 3 Jun 2026](https://finance.yahoo.com/sectors/energy/articles/many-airlines-hit-hard-jet-124112122.html), [Aviation Business News](https://aviationbusinessnews.com/low-cost/latest-news-low-cost/wide-fuel-margins-keep-pressure-on-lcc-hedge-books) |
| Singapore jet crack, March 2026 | peak 79, falling back to 40 within a week | [Yahoo Finance, 11 Mar 2026](https://finance.yahoo.com/news/europe-faces-jet-fuel-price-000000069.html) |
| European jet-diesel regrade | normally -5 to +2 US$/bbl; 45-48 after the war began | same |
| USGC crack, EIA | US$1.25/gal (52.5/bbl) average March-May 2026 against US$0.42/gal (17.6/bbl) at the start of the year | [EIA, 8 Jun 2026](https://www.eia.gov/todayinenergy/detail.php?id=67764) |
| 2022 European jet barge crack | record 69.37 US$/bbl on 29 Apr 2022 (Platts, via search summary of an S&P Global article) | [S&P Global, 9 May 2022](https://www.spglobal.com/energy/en/news-research/latest-news/crude-oil/050922-jet-fuel-reclaims-king-of-the-barrel-status-in-europe-amid-demand-revival) |
| IATA on 2022 | jet rose about 75% to US$136/bbl; crack was about 35% of crude, expected to narrow to 23%, "still a little above the long-run average" | [IATA fuel fact sheet, June 2023](https://www.iata.org/en/iata-repository/publications/economic-reports/fuel-fact-sheet) |
| Refining margin cost in 2008 | the wider margin alone added US$34.5bn to the industry fuel bill | [IATA briefing, Feb 2010](https://www.iata.org/en/iata-repository/publications/economic-reports/airline-fuel-labourcosts) |

Working rule: in calm markets the crack is 17-20% of Brent, about 13-18 US$/bbl at US$70-90 crude. In a refining or logistics crisis it runs at 40-70 for months, with daily prints above 100 at a single hub.

Mechanism. Jet competes with diesel for the middle-distillate cut. Diesel drives refinery economics, so a diesel shortage pulls jet with it; IMO 2020 raised the floor under middle distillates; refinery closures in developed economies made importing regions fragile (IATA chart of the week, above). In 2026 Europe took 24% and Africa 33% of their jet from the Persian Gulf, and European jet output was 13% below 2019 (UK 42% below) (IATA briefing, above).

### 1.4 Regional differentials

EIA publishes only the Gulf Coast jet series; its New York Harbor and Los Angeles jet pages no longer resolve. Singapore and Rotterdam assessments are Platts and Argus proprietary. What can be sourced:

| Evidence | Numbers | Source |
|---|---|---|
| IATA regional prices, calm week (30 Jan 2026), US$/bbl | Global 94.97; Asia & Oceania 86.85; Europe & CIS 96.37; Middle East 82.61; North America 99.96; Latin & Central America 98.31; Africa 92.68. Region weights: North America 39%, Europe 28%, Asia 22%, Middle East 5%, Latin America 4%, Africa 2% | [Aviation Week datasheet](https://aviationweek.com/system/files/datasheets/jet-fuel-price-index-week-ending-jan-30-2026.pdf) |
| Same, crisis week (21 Aug 2026) | Global 163.87; Asia & Oceania 155.69; Europe 169.68; Middle East 151.62; North America 164.73; Latin America 172.53 | [Gulf Times](https://www.gulf-times.com/article/731943/business/middle-east-maintains-lowest-global-jet-fuel-price-despite-80-annual-rise-iata) |
| Same, 18 Sep 2026 | Global 194.65; US 193.65; Europe 207.56 | [Leeham](https://leehamnews.com/2026/09/28/soaring-fuel-prices-dim-end-of-year-outlook-for-major-u-s-airlines/) |
| Spread across four hubs (Singapore, NWE, US West Coast, US Gulf Coast) | 2-41 US$/bbl between June 2025 and February 2026; 9-85 US$/bbl after the war began | [IATA briefing](https://www.iata.org/en/iata-repository/pressroom/presentations/media-briefing-jet-fuel-update-agm-2026/) |
| New York Harbor record | US$7.61/gal on 4 Apr 2022 when Gulf Coast was US$3.811 (a premium of US$3.80/gal, OWN CALC for the Gulf Coast leg); East Coast stocks 6.5m bbl in the week to 8 Apr 2022, the lowest since 1990; cause: Philadelphia refinery closure (335,000 b/d, mid-2019) plus post-COVID yield shifts | [EIA Today in Energy, 11 May 2022](https://www.eia.gov/todayinenergy/detail.php?id=52378), [reprint with the price](https://www.ajot.com/news/east-coast-jet-fuel-prices-remain-higher-than-other-spot-price-hubs), [S&P Global](https://www.spglobal.com/energy/en/news-research/latest-news/crude-oil/040422-jet-fuel-for-delivery-to-new-york-airports-hits-643gal-or-270b-a-likely-spot-market-record) |
| Freight, China to ARA | 8-9 US$/bbl in March 2026 against 4-5 before | [Yahoo Finance](https://finance.yahoo.com/news/europe-faces-jet-fuel-price-000000069.html) |
| Local-currency effect, April 2025 to April 2026 peak | global jet +121%; Japan +173%; India +170%; China +132%; euro area +118%; Brazil and Mexico +86% | [AFM, 1 Sep 2026](https://afm.aero/iata-reports-121percent-global-jet-fuel-price-shock-in-2026-with-india-and-japan-most-exposed) |

Diesel as a proxy for US regional jet basis (ultra-low-sulfur diesel spot, same hubs, OWN CALC, c/gal):

| Basis | Period | Mean | Median | 5th-95th pct | Min / max | Half-life | Daily innovation sd |
|---|---|---|---|---|---|---|---|
| New York Harbor minus Gulf Coast | 2010-19 | 5.5 | 4.3 | 1.7 to 12.2 | -6.0 / 49.2 | 11 trading days | 1.6 |
| Los Angeles minus Gulf Coast | 2010-19 | 7.5 | 7.0 | -0.7 to 17.2 | -7.5 / 45.0 | 9 days | 2.3 |
| New York Harbor minus Gulf Coast | 2021-26 | 10.5 | 7.0 | 1.8 to 27.2 | -19.5 / 140.3 | 9 days | 5.6 |
| Los Angeles minus Gulf Coast | 2021-26 | 12.4 | 10.5 | -10.2 to 46.5 | -113.0 / 80.3 | 8 days | 7.1 |
| Los Angeles minus Gulf Coast | Mar-Nov 2026 | 26.2 | 23.8 | -3.3 to 67.2 | -21.5 / 80.3 | 4 days | 10.3 |
| Gulf Coast jet minus Gulf Coast diesel | 2010-19 | -2.5 | -2.5 | -9.3 to 4.5 | -16.5 / 22.5 | 10 days | 1.5 |

Reading of the three IATA snapshots, relative to the global average in US$/bbl: Asia & Oceania about -8 in both calm and crisis; Middle East about -12 in both; Europe +1.5 calm, +6 to +13 crisis; North America +5 calm (winter), 0 to -1 crisis; Latin America +3.5 calm, +9 crisis; Africa -2.

The same snapshots against the EIA Gulf Coast average for the same weeks (OWN CALC: US$91.74/bbl for the week to 30 January, 164.73 for the week to 21 August, 192.52 for the week to 18 September 2026):

| Region minus US Gulf Coast | Calm week, US$/bbl (c/gal) | Crisis weeks, US$/bbl (c/gal) |
|---|---|---|
| IATA global average | +3.2 (+8) | -0.9 and +2.1 (-2, +5) |
| IATA North America | +8.2 (+20) | 0.0 and +1.1 (0, +3) |
| Europe | +4.6 (+11) | +5.0 and +15.0 (+12, +36) |
| Asia & Oceania | -4.9 (-12) | -9.0 (-21) |
| Middle East | -9.1 (-22) | -13.1 (-31) |
| Latin America | +6.6 (+16) | +7.8 (+19) |
| Africa | +0.9 (+2) | not reported |

The Gulf Coast spot series therefore tracks IATA's global average within about US$3/bbl, and IATA's weekly crack matches the Gulf Coast crack in the two crisis weeks (70.9 against 70.5; 67.1 against 68.4). Two or three data points per region only, so treat the regional means in section 9 as ESTIMATE.

### 1.5 From spot to into-plane

| Item | Value | Source |
|---|---|---|
| US carriers' average paid price minus Gulf Coast spot, years with little hedging noise | +12 c/gal (2019), +15 (2023), +20 (2024), +22 (2025); +10 in 2012 | OWN CALC: DOT column minus EIA column in 1.2 |
| Timing lag in 2026 | DOT paid price by month: Jan 2.35, Feb 2.39, Mar 3.17, Apr 4.11, May 4.09, Jun 3.59, Jul 3.40, Aug 3.72, Sep 3.86. Spot jumped 63% in March but the paid price rose 33%. "Average of this month's and last month's Gulf Coast spot, plus an adder" fits January to August with adders of 35, 24, 19, 30, 15, 10, 18 and 16 c/gal; September (preliminary data) comes out 20 c/gal below, so the true lag may be longer in a rising market | [BTS](https://www.transtats.bts.gov/fuel.asp); fit is OWN CALC |
| US federal tax on commercial jet fuel | 4.3 c/gal excise plus 0.1 c/gal LUST = 4.4 c/gal | [Airlines for America](https://www.airlines.org/dataset/government-imposed-taxes-on-air-transportation/) |
| Into-plane fee, large hydrant hub | "a few cents per gallon", materially higher where the carrier has no volume | vendor commentary via search summary; low quality |
| Into-plane fee ranges quoted by vendors | 10-40 c/gal; 10-85 c/gal in the Gulf region, set by airport and bargaining position | [LFS journal](https://lfs.aero/journal/jet-fuel-prices-gulf-2026-platts-explained) and others via search summary; low quality |
| Price spread between European airports | 30% between Amsterdam and Ibiza or Hamburg and Oslo; 20% between Heathrow and Glasgow (negotiated prices of two major airlines at 140 airports) | [EUROCONTROL think paper 1, 2019](https://www.eurocontrol.int/sites/default/files/2020-01/eurocontrol-think-paper-1-fuel-tankering.pdf) |
| Ryanair's definition of the fuel line | direct cost of fuel, the cost of delivering fuel to the aircraft, and (per its accounting note) EU ETS carbon credits; de-icing also sits there (RECALLED) | [Ryanair 2026 annual report](https://investor.ryanair.com/wp-content/uploads/2026/06/Ryanair-2026-Annual-Report.pdf) |
| Places with high differentials | India (state VAT on domestic ATF), Brazil, parts of Africa, islands | RECALLED, UNVERIFIED; no figures retrieved |

Structure of a supply contract (RECALLED, UNVERIFIED in detail; consistent with the lag fit above): price = hub index (Platts or Argus mean for the prior week, fortnight or month) + location differential (pipeline or barge freight, terminal, losses) + into-plane fee + taxes. The airline negotiates the differential and the fee, never the index.

---

## 2. Volatility and how to model it

### 2.1 Measured volatility

All OWN CALC from EIA daily spot, log returns, 252 trading days per year.

| Statistic | USGC jet | Brent |
|---|---|---|
| Annualised volatility of daily returns, 1990-2026 | 46% | 44% |
| Same, 2010-2019 (calm decade) | 30% | 30% |
| Calendar-year range | 19% (2013) to 76% (2020); above 65% in 1990, 2000, 2005, 2008, 2020, 2022, 2026 | 18% (2013, 2014) to 114% (2020) |
| Diffusion-only volatility after 3-sigma clipping | 34.5% (25.6% in 2010-19) | 31.3% (26.4% in 2010-19) |
| Clipped "jump" days per year | 9.1 (5.5 in 2010-19) | 8.7 (5.6 in 2010-19) |
| Jump size sd | 10.3% (6.9% in 2010-19), mean about zero, 48% up | 10.6% (6.5% in 2010-19) |
| Excess kurtosis indicator (raw kurtosis) | 16 | 56 (driven by 2020) |
| Volatility of monthly-average prices, annualised | 34.6% | 35.4% |
| Correlation of daily returns, jet vs Brent | 0.60 | |
| Correlation of monthly-average returns | 0.84 | |

Distribution of the change in the monthly-average jet price over a horizon (all starting months 1990-2026), OWN CALC:

| Horizon | 5th pct | 25th | Median | 75th | 95th | Min | Max |
|---|---|---|---|---|---|---|---|
| 3 months | -25% | -8% | +2% | +13% | +36% | -66% | +122% |
| 6 months | -34% | -10% | +3% | +21% | +55% | -67% | +120% |
| 12 months | -44% | -14% | +2% | +36% | +84% | -69% | +175% |

Named moves in monthly averages (OWN CALC): +124% Jul to Oct 1990; -65% Jul to Dec 2008; -68% Jun 2014 to Jan 2016; -66% Jan to Apr 2020; +68% Jan to Jun 2022; +63% Feb to Mar 2026 and +98% Feb to Oct 2026.

### 2.2 Fitted mean-reversion parameters

AR(1) on daily log prices, converted to continuous-time Ornstein-Uhlenbeck parameters (dX = kappa (mu - X) dt + sigma dW, X = ln price). OWN CALC.

| Series and window | kappa per year | Half-life | exp(mu) | sigma (annual) |
|---|---|---|---|---|
| ln USGC jet, 1990-2026 | 0.23 | 3.0 years | US$1.81/gal | 0.46 |
| ln USGC jet, 1990-2003 | 1.83 | 0.38 years | US$0.61/gal | 0.48 |
| ln USGC jet, 2005-2026 | 0.81 | 0.85 years | US$2.27/gal | 0.45 |
| ln USGC jet, 2005-2019 | 0.86 | 0.80 years | US$2.16/gal | 0.38 |
| ln USGC jet, 2015-2025 | 0.88 | 0.79 years | US$1.90/gal | 0.47 |
| ln Brent, 1990-2026 | 0.22 | 3.2 years | US$57.7/bbl | 0.44 |
| ln Brent, 2005-2026 | 0.91 | 0.76 years | US$75.9/bbl | 0.44 |
| ln Brent, 2015-2025 | 1.25 | 0.55 years | US$64.1/bbl | 0.48 |
| ln(jet/Brent), 1990-2019 | 15.9 | 11 trading days | ratio 1.21 | 0.40 |
| ln(jet/Brent), 2005-2019 | 12.7 | 14 trading days | ratio 1.19 | 0.35 |
| ln(jet/Brent), 2021-2026 | 5.0 | 35 trading days | ratio 1.32 | 0.44 |
| Crack in US$/bbl, 1990-2004 | 15.7 | 11 trading days | mean 5.5 | daily sd 0.86 |
| Crack in US$/bbl, 2006-2019 | 13.9 | 13 trading days | mean 14.2 | daily sd 1.93 |
| Crack in US$/bbl, 2021-2026 | 4.5 | 39 trading days | mean 29.8 | daily sd 3.64 |

Three things follow. First, the full-sample fit shows almost no reversion because the level shifted around 2004; within a regime the half-life is 7-10 months. A single OU around a fixed mean is therefore wrong; use a slow-moving equilibrium plus a fast deviation, which is the Schwartz-Smith structure. Second, the crack reverts ten to twenty times faster than the crude level, so model it as its own factor. Third, the daily crack sigma contains single-hub spot noise that reverses within days; an airline paying a monthly-average index does not see it.

### 2.3 Crude as a proxy hedge: measured basis risk

Regression of the change in the monthly-average jet price (US$/bbl) on the change in monthly-average Brent, OWN CALC:

| Horizon | Correlation | R-squared | Beta | Residual sd, US$/bbl |
|---|---|---|---|---|
| 1 month | 0.80 | 0.63 | 1.10 | 4.9 |
| 3 months | 0.90 | 0.81 | 1.19 | 6.7 |
| 6 months | 0.93 | 0.86 | 1.22 | 7.9 |
| 12 months | 0.94 | 0.88 | 1.29 | 10.0 |
| 3 months, 2021-2026 only | 0.87 | 0.75 | 1.44 | - |
| 12 months, 2021-2026 only | 0.92 | 0.85 | 1.75 | - |

A barrel-for-barrel Brent hedge removes 80-88% of jet price variance over a quarter to a year in the full sample and leaves a residual of US$7-10/bbl. Since 2021 the beta is 1.4-1.75: jet moves much more than crude, so a one-for-one crude hedge under-hedges exactly when it matters. IATA makes the same point in words: the historical correlation "has weakened in recent years", so managing fuel exposure through crude alone no longer works ([IATA, 11 Sep 2026](https://www.iata.org/en/iata-repository/publications/economic-reports/structural-shifts-increase-jet-fuel-crack-risks/)).

### 2.4 How commodity practitioners model it

| Model | Form | Use | Published oil estimates |
|---|---|---|---|
| Schwartz (1997) one-factor | d ln S = kappa (mu - ln S) dt + sigma dW | Simplest mean-reverting spot | RECALLED, UNVERIFIED: oil futures 1985-95 kappa 0.301, mu 3.093, sigma 0.334; 1990-95 kappa 0.694, mu 3.037, sigma 0.326; Enron long-dated data 1993-96 kappa 0.428, mu 2.991, sigma 0.257. Journal of Finance 52(3), 923-973 |
| Schwartz (1997) two- and three-factor | Adds stochastic convenience yield, then stochastic interest rate | Fitting the futures curve | not recalled |
| Schwartz-Smith (2000) two-factor | ln S = chi + xi; d chi = -kappa chi dt + sigma_chi dW1; d xi = mu_xi dt + sigma_xi dW2; corr rho | Separates transient deviations from a random-walk equilibrium | RECALLED, UNVERIFIED: futures 1990-95 kappa 1.49, sigma_chi 28.6%, sigma_xi 14.5%, mu_xi -1.25%, rho 0.30; Enron data 1993-96 kappa 1.19, sigma_chi 15.8%, sigma_xi 11.5%, rho 0.19. Management Science 46(7), 893-911 |
| Merton jump-diffusion overlay | Adds Poisson jumps with normal log sizes | Fat tails; event days | See 2.1 for jump statistics fitted here |
| Regime switching | Markov states with different mean and variance | Crisis versus normal crack | See 9(a) for the regime frequencies observed here |
| Spread model | Second OU factor for the crack or for ln(jet/crude) | Jet as a derived product | Fits in 2.2 |
| Implied volatility | Black-76 on NYMEX options gives the market's forward sd | EIA publishes 95% bands this way | [EIA STEO supplement, Oct 2009](https://www.eia.gov/outlooks/steo/special/pdf/2009_sp_05.pdf) |

The two recalled parameter sets were not re-opened in this session. They agree with the fits above: a short-term factor with half-life about half a year and volatility near 30%, and a persistent factor with volatility near 15%.

---

## 3. Fuel share of operating cost

### 3.1 Industry

| Year | IATA fuel share of operating expenses | Source |
|---|---|---|
| 2001-2003 | 12-13% (13.6% in 2001 for a 45-airline sample) | [IATA briefing, Feb 2010](https://www.iata.org/en/iata-repository/publications/economic-reports/airline-fuel-labourcosts) |
| 2004 / 2005 / 2006 / 2007 | 17.3 / 22.2 / 28.4 / 29.8% | [IATA mid-2018 table](https://www.iata.org/en/iata-repository/publications/economic-reports/airline-industry-economic-performance---2018-mid-year---table) |
| 2008 | 35.6% (32.3% in the 45-airline sample; 36% in the [mid-2015 table](https://www.iata.org/en/iata-repository/publications/economic-reports/airline-industry-economic-performance---2015-mid-year---tables)) | same |
| 2009 / 2010 / 2011 | 28.3 / 28.3 / 30.7% | same |
| 2012 / 2013 / 2014 | 33.2 / 33.2 / 30.6% | same |
| 2015 / 2016 / 2017 | 26.4 / 20.8 / 21.4% | same |
| 2019 / 2020 / 2021 | 23.9 / 16.1 / 19.0% | [IATA industry statistics, June 2026](https://www.iata.org/en/iata-repository/pressroom/fact-sheets/industry-statistics/) |
| 2022 / 2023 / 2024 | 29.6 / 31.2 / 27.8% | same |
| 2025E / 2026F | 25.4 / 31.4% (fuel bill US$252bn to US$350bn; consumption 104bn gallons both years) | same |

Regional split in 2008: fuel was 34.2% of cost for North American carriers, 25.3% for European, 36.7% for Asia-Pacific, and above 50% for some Asian carriers; in 2001 the same figures were 13.4%, 12.2%, 15.7% (IATA briefing, Feb 2010). Labour fell from 28.3% to 20.1% over the same years.

Fuel consumption is flat against price: 66bn gallons in 2004, 70bn in 2008, 96bn in 2019, 52bn in 2020, 104bn in 2025 (IATA tables). The bill moves with price, not volume.

### 3.2 US carriers

US DOT Form 41 cost per gallon is the last column of the table in 1.2 (2000: US$0.80; 2008: US$3.06; 2012: US$3.16; 2016: US$1.45; 2020: US$1.43; 2022: US$3.37; 2025: US$2.33; Jan-Sep 2026: US$3.44). US scheduled carriers burned 19.0bn gallons in 2000, 18.0bn in 2008, 15.9bn in 2012, 18.3bn in 2019, 10.3bn in 2020 and 19.0bn in 2025 ([BTS](https://www.transtats.bts.gov/fuel.asp)).

| Carrier | Year | US$/gal paid | Fuel % of operating expense | Source |
|---|---|---|---|---|
| Southwest | 1999 | 0.53 | 12.5% | [FY1999 10-K](https://www.sec.gov/Archives/edgar/data/0000092380/000095013400002530/0000950134-00-002530.txt) |
| Southwest | 2004 / 2005 / 2006 / 2007 / 2008 | 0.92 / 1.13 / 1.64 / 1.80 / 2.44 (incl. tax, net of hedges) | 18.1 / 21.4 / 28.0 / 29.7 / 35.1% | [FY2008 10-K](https://sec.gov/Archives/edgar/data/92380/000119312509015591/d10k.htm) |
| Southwest | 2011-2014 | 3.25 / 3.32 / 3.19 / 2.97 | 38.2 / 37.3 / 35.3 / 32.6% | [FY2025 10-K](https://www.sec.gov/Archives/edgar/data/92380/000009238026000004/luv-20251231.htm) |
| Southwest | 2015-2019 | 1.96 / 1.90 / 1.99 / 2.20 / 2.09 | 23.6 / 22.7 / 23.0 / 24.6 / 22.3% | same |
| Southwest | 2020-2025 | 1.45 / 1.98 / 3.10 / 2.89 / 2.64 / 2.41 | 14.4 / 23.5 / 26.2 / 24.0 / 21.4 / 19.0% | same |
| American (AMR) | 2006 / 2007 / 2008 | 2.014 / 2.131 / 3.034 | 29.8 / 30.4 / 35.1% | [AMR FY2008 10-K](https://www.sec.gov/Archives/edgar/data/6201/000000620109000009/ar120810k.htm) |
| American | 2014 / 2015 (mainline) | 2.91 / 1.72 | 33.2 / 21.6% | [AAL FY2015 10-K](https://www.sec.gov/Archives/edgar/data/6201/000119312516474605/d78287d10k.htm) |
| United | 2006 / 2007 / 2008 (system) | 2.125 / 2.217 / 3.517 | - | [UAL FY2008 10-K](https://www.sec.gov/Archives/edgar/data/100517/000095013709001460/c49215e10vk.htm) |
| United | 2015 / 2016 / 2017 | 1.94 / 1.49 / 1.74 | 23 / 18 / 20% | [UAL FY2017 10-K](https://www.sec.gov/Archives/edgar/data/100517/000119312518054235/d471340d10k.htm) |
| Delta | 2006 / 2007 / 2008 | 2.12 / 2.24 / 3.16 | 30 / 31 / 38% (28% including impairments) | [DAL FY2008 10-K](https://www.sec.gov/Archives/edgar/data/27904/000119312509042726/d10k.htm) |
| Delta | 2010 / 2011 / 2012 | 2.33 / 3.06 / 3.25 | 30 / 36 / 36% | [DAL FY2012 10-K](https://www.sec.gov/Archives/edgar/data/27904/000144530513000209/dal1231201210k.htm) |
| Delta | 2015 / 2016 / 2017 | 1.90 / 1.49 / 1.68 | 23.0 / 18.3 / 19.2% | [DAL FY2017 10-K](https://www.sec.gov/Archives/edgar/data/27904/000002790418000006/dal1231201710k.htm) |
| Delta | 2023 / 2024 / 2025 | 2.82 / 2.57 / 2.30 | 21 / 19 / 17% | [DAL FY2025 10-K](https://www.sec.gov/Archives/edgar/data/27904/000002790426000013/dal-20251231.htm) |
| Ryanair | FY2025 / FY2026 | EUR 3.02 / 3.01 per US gal | about 42 / 41% | [Ryanair FY2026 20-F](https://www.sec.gov/Archives/edgar/data/1038683/000110465926076131/tmb-20260331x20f.htm), [annual report](https://investor.ryanair.com/wp-content/uploads/2026/06/Ryanair-2026-Annual-Report.pdf) |

Sensitivity: American, with no hedges, stated that one cent per gallon changes its annual fuel expense by US$44m (FY2015 10-K). Fuel share scales inversely with the rest of the cost base: an ultra-low-cost carrier sits at about 40% when a US network carrier sits at 17-20% at the same fuel price.

---

## 4. Fuel burn by aircraft type

### 4.1 Source and method

The best public per-type data is the EUROCONTROL "master emissions calculator" distributed with the EMEP/EEA air pollutant emission inventory guidebook 2023, chapter 1.A.3.a ([spreadsheet](https://www.eea.europa.eu/en/analysis/publications/emep-eea-guidebook-2023/part-b-sectoral-guidance-chapters/1-energy/1-a-combustion/1-a-3-a-aviation.3), [chapter](https://www.eea.europa.eu/en/analysis/publications/emep-eea-guidebook-2023/part-b-sectoral-guidance-chapters/1-energy/1-a-combustion/1-a-3-a-aviation-2023)). For 283 aircraft types it gives fuel for one ICAO landing and take-off cycle (LTO: 32.9 minutes, of which 26 are taxi) and fuel plus duration for climb, cruise and descent (CCD) at stage lengths of 125, 200, 250, 500, 750, 1,000 nm and then every 500 nm to the type's range. It is generated with EUROCONTROL's Advanced Emission Model on BADA performance data, baseline year 2022, and uses 3.15 kg CO2 per kg fuel.

I fitted, per type, trip fuel (kg) = LTO + a + b x d (linear) and LTO + a + b x d + c x d^2 (quadratic), with d the great-circle stage in km. All coefficients below are OWN CALC from that database. The quadratic term captures the fuel burned carrying fuel, which matters beyond about 3,000 nm. The linear fit's worst error is at 125 nm for widebodies, a stage they do not fly.

### 4.2 Coefficients

Columns: LTO fuel; linear intercept a and slope b; quadratic a, b, c; marginal cruise burn (from 500 nm to maximum stage); average burn per block hour at the example stage (CCD time plus 26 minutes taxi); example trip fuel including LTO; then per-seat figures at the seat count shown. Seat counts are ESTIMATE (typical two-class or dense single-class layouts); the per-seat columns move in proportion if you change them. US gal per block hour uses 3.04 kg/gal.

| ICAO | Type | LTO kg | Linear: a kg | b kg/km | Quadratic: a kg | b kg/km | c kg/km2 x1e-5 | Cruise kg/h | kg/block-h | Example trip | kg/km | Seats (assumed) | g/seat-km | L/100 seat-km | USgal/block-h |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| AT76 | ATR 72-600 | 199 | 56 | 1.447 | - | - | - | 639 | 647 | 915 kg @ 250 nm | 1.98 | 70 | 28.2 | 3.53 | 213 |
| DH8D | Dash 8 Q400 | 259 | 133 | 1.711 | 150 | 1.660 | 2.50 | 1100 | 925 | 1171 kg @ 250 nm | 2.53 | 78 | 32.4 | 4.05 | 304 |
| CRJ9 | CRJ900 | 480 | 184 | 1.898 | 140 | 1.997 | -3.42 | 1589 | 1482 | 2416 kg @ 500 nm | 2.61 | 76 | 34.3 | 4.29 | 488 |
| E75L | E175 | 482 | 235 | 1.703 | 167 | 1.830 | -3.38 | 1323 | 1353 | 2278 kg @ 500 nm | 2.46 | 76 | 32.4 | 4.05 | 445 |
| E190 | E190 | 596 | 185 | 2.235 | 236 | 2.142 | 2.48 | 1803 | 1679 | 2809 kg @ 500 nm | 3.03 | 100 | 30.3 | 3.79 | 552 |
| BCS1 | A220-100 | 598 | 294 | 2.406 | 304 | 2.392 | 0.26 | 1997 | 1925 | 5314 kg @ 1000 nm | 2.87 | 120 | 23.9 | 2.99 | 633 |
| BCS3 | A220-300 | 530 | 313 | 2.635 | 348 | 2.587 | 0.87 | 2191 | 2053 | 5662 kg @ 1000 nm | 3.06 | 140 | 21.8 | 2.73 | 675 |
| A319 | A319ceo | 689 | 233 | 2.569 | 389 | 2.376 | 3.05 | 2143 | 2023 | 5566 kg @ 1000 nm | 3.01 | 140 | 21.5 | 2.68 | 665 |
| A320 | A320ceo | 816 | 274 | 2.773 | 301 | 2.736 | 0.68 | 2311 | 2251 | 6192 kg @ 1000 nm | 3.34 | 170 | 19.7 | 2.46 | 740 |
| A321 | A321ceo | 1075 | 210 | 3.551 | 373 | 3.350 | 3.18 | 2973 | 2812 | 7744 kg @ 1000 nm | 4.18 | 200 | 20.9 | 2.61 | 925 |
| A19N | A319neo | 661 | 182 | 2.422 | 314 | 2.261 | 2.57 | 2028 | 1912 | 5260 kg @ 1000 nm | 2.84 | 140 | 20.3 | 2.54 | 629 |
| A20N | A320neo | 661 | 227 | 2.414 | 327 | 2.291 | 1.96 | 2016 | 1922 | 5309 kg @ 1000 nm | 2.87 | 180 | 15.9 | 1.99 | 632 |
| A21N | A321neo | 749 | 245 | 2.866 | 300 | 2.805 | 0.85 | 2441 | 2289 | 6280 kg @ 1000 nm | 3.39 | 210 | 16.1 | 2.02 | 753 |
| B737 | 737-700 | 825 | 245 | 2.598 | 295 | 2.544 | 0.77 | 2149 | 2110 | 5821 kg @ 1000 nm | 3.14 | 137 | 22.9 | 2.87 | 694 |
| B738 | 737-800 | 825 | 319 | 2.850 | 396 | 2.743 | 1.94 | 2386 | 2305 | 6382 kg @ 1000 nm | 3.45 | 175 | 19.7 | 2.46 | 758 |
| B38M | 737 MAX 8 | 731 | 258 | 2.415 | 336 | 2.319 | 1.52 | 2033 | 1980 | 5431 kg @ 1000 nm | 2.93 | 175 | 16.8 | 2.09 | 651 |
| B733 | 737-300 | 783 | 256 | 2.924 | 303 | 2.836 | 2.33 | 2340 | 2223 | 3720 kg @ 500 nm | 4.02 | 130 | 30.9 | 3.86 | 731 |
| MD82 | MD-82 | 985 | 264 | 3.451 | 296 | 3.392 | 1.57 | 2819 | 2659 | 4409 kg @ 500 nm | 4.76 | 150 | 31.7 | 3.97 | 875 |
| B752 | 757-200 | 1363 | 205 | 4.164 | 330 | 4.010 | 2.45 | 3572 | 3398 | 9206 kg @ 1000 nm | 4.97 | 190 | 26.2 | 3.27 | 1118 |
| B763 | 767-300(ER) | 1730 | 545 | 5.457 | 519 | 5.478 | -0.21 | 4614 | 4630 | 42821 kg @ 4000 nm | 5.78 | 240 | 24.1 | 3.01 | 1523 |
| A332 | A330-200 | 2168 | 569 | 6.178 | 740 | 6.070 | 0.89 | 5354 | 5327 | 48272 kg @ 4000 nm | 6.52 | 260 | 25.1 | 3.13 | 1752 |
| A333 | A330-300 | 2168 | 787 | 5.876 | 860 | 5.822 | 0.51 | 5093 | 5155 | 46710 kg @ 4000 nm | 6.31 | 290 | 21.7 | 2.72 | 1696 |
| A339 | A330-900 | 1817 | 108 | 6.152 | 848 | 5.683 | 3.83 | 5381 | 5180 | 46935 kg @ 4000 nm | 6.34 | 290 | 21.8 | 2.73 | 1704 |
| A343 | A340-300 | 2020 | 76 | 7.786 | 578 | 7.468 | 2.60 | 6607 | 6387 | 59178 kg @ 4000 nm | 7.99 | 270 | 29.6 | 3.70 | 2101 |
| B788 | 787-8 | 1638 | 170 | 5.338 | 651 | 5.055 | 2.16 | 4778 | 4611 | 40813 kg @ 4000 nm | 5.51 | 240 | 23.0 | 2.87 | 1517 |
| B789 | 787-9 | 1638 | 276 | 5.702 | 677 | 5.482 | 1.58 | 5064 | 4945 | 43812 kg @ 4000 nm | 5.91 | 290 | 20.4 | 2.55 | 1627 |
| A359 | A350-900 | 2138 | 207 | 6.726 | 797 | 6.402 | 2.32 | 6089 | 5879 | 51892 kg @ 4000 nm | 7.00 | 315 | 22.2 | 2.78 | 1934 |
| A35K | A350-1000 | 2412 | 353 | 7.496 | 932 | 7.198 | 2.00 | 6726 | 6554 | 57847 kg @ 4000 nm | 7.81 | 360 | 21.7 | 2.71 | 2156 |
| B772 | 777-200(ER) | 2168 | 317 | 7.053 | 850 | 6.739 | 2.39 | 6242 | 6114 | 54235 kg @ 4000 nm | 7.32 | 300 | 24.4 | 3.05 | 2011 |
| B77W | 777-300ER | 2907 | 278 | 9.214 | 791 | 8.889 | 2.66 | 8309 | 8018 | 70988 kg @ 4000 nm | 9.58 | 365 | 26.3 | 3.28 | 2638 |
| B744 | 747-400 | 3908 | 665 | 10.688 | 973 | 10.507 | 1.38 | 9558 | 9464 | 83536 kg @ 4000 nm | 11.28 | 400 | 28.2 | 3.52 | 3113 |
| B748 | 747-8 | 3495 | 965 | 11.735 | 991 | 11.720 | 0.12 | 10459 | 10245 | 90334 kg @ 4000 nm | 12.19 | 410 | 29.7 | 3.72 | 3370 |
| A388 | A380-800 | 3859 | 128 | 14.541 | 1320 | 13.885 | 4.69 | 13109 | 12612 | 110500 kg @ 4000 nm | 14.92 | 500 | 29.8 | 3.73 | 4149 |

Notes on the database. The 767, 777 and 737-700 rows are EUROCONTROL's generic B763, B772 and B737 entries, which cover the ER variants. B788 and B789 share one LTO value. There is no separate E175 code; E75L is the E175 with the long wing. The 737 MAX 9, 787-10, 767-400 and 777-200LR are in the database too (MAX 9: LTO 731 kg, linear a 234 kg, b 2.569 kg/km; 787-10: 1,868, 667, 5.825; 767-400: 1,734, 409, 5.814; 777-200LR: 2,907, 821, 8.612), as are about 250 other types.

### 4.3 Cross-check against manufacturer and airline figures

Wikipedia's table collects manufacturer and trade-press figures per type and sector ([Fuel economy in aircraft](https://en.wikipedia.org/wiki/Fuel_economy_in_aircraft), each row cited there). Selected rows:

| Type | Sector | Seats | kg/km | L/100 km per seat |
|---|---|---|---|---|
| ATR 72-600 | 300 nm / 500 nm | 72 | 1.56 / 1.41 | 2.79 / 2.46 |
| Dash 8 Q400 | 300 nm | 78 | 2.16 | 3.46 |
| E175 | 605 nm | 88 | 2.80 | 3.97 |
| E175-E2 / E190-E2 / E195-E2 | 600 nm | 88 / 106 / 132 | 2.44 / 2.83 / 3.07 | 3.44 / 3.32 / 2.90 |
| A220-100 / A220-300 | 1,000 nm | 125 / 160 | 2.28 / 2.56 | 2.28 / 2.00 |
| A319 / A320 / A321-200 | 1,000 nm | 124 / 150 / 180 | 2.93 / 3.13 / 3.61 | 2.95 / 2.61 / 2.50 |
| A319neo / A320neo / A321neo | 1,000 nm | 136 / 180 / 220 | 2.40 / 2.79 / 3.47 | 2.22 / 1.94 / 1.98 |
| 737-700 / 737-800 / 737-900ER | 1,000 nm | 126 / 162 / 180 | 2.82 / 3.17 / 3.42 | 2.79 / 2.44 / 2.38 |
| 737 MAX 7 / 8 / 9 | 1,000 nm | 140 / 162 / 180 | 2.51 / 2.71 / 2.91 | 1.94 / 2.04 / 2.02 |
| 757-200 / 757-300 | 1,000 nm | 200 / 243 | 4.16 / 4.68 | 2.59 / 2.40 |
| 767-300ER | 3,000 nm | 218 / 269 | 5.39 / 5.51 | 3.09 / 2.56 |
| A330-200 / A330-300 | 3,000 nm | 241 / 262 | 6.0 / 6.25 | 3.11 / 2.98 |
| A330-900 | 4,650 nm | 300 | 5.94 | 2.48 |
| 787-8 / 787-9 | 4,650 nm | 243 / 294 | 5.38 / 5.85 | 2.77 / 2.49 |
| A350-900 | 4,972 / 6,542 nm | 315 | 6.03 / 7.07 | 2.39 / 2.81 |
| A350-1000 | 5,531 nm | 367 | 7.58 | 2.58 |
| 777-200ER | 6,000 nm | 301 | 7.42 | 3.08 |
| 777-300ER | 6,000 / 7,200 nm | 365 / 344 | 8.49 / 8.58 | 2.91 / 3.11 |
| 747-400 | 6,000 nm | 416 | 11.11 | 3.34 |
| 747-8 | 6,000 nm | 467 | 10.54 | 2.82 |
| A380 | 6,000 / 7,200 nm | 544 / 525 | 13.78 / 13.78 | 3.16 / 3.27 |

The EUROCONTROL fits run 0-12% above the manufacturer-style numbers (A320neo 2.87 against 2.79 kg/km; MAX 8 2.93 against 2.71; 777-300ER 9.58 against 8.5-8.9) because they include a full 26-minute taxi and operational profiles instead of brochure assumptions. That is the right bias for a simulator. The A350-900 row shows the range effect: 6.03 kg/km at 4,972 nm and 7.07 at 6,542 nm.

### 4.4 Operating adjustments

All from Airbus, "Getting to grips with fuel economy", issue III, July 2004 ([copy hosted by EUROCONTROL](https://ansperformance.eu/library/airbus-fuel-economy.pdf)) unless stated.

| Effect | Number |
|---|---|
| Taxi burn, 12 minutes, all engines | A318/A319 120 kg; A320 138 kg; A321 162 kg; A330 300 kg; A340-200/300 300 kg; A340-500/600 420 kg. So 10-13.5 kg/min narrowbody, 25-35 kg/min widebody. Single-engine taxi saves about a third. |
| Weight | +1% of MTOW costs about 1% of specific range (up to 1.5% at high altitude) |
| Fuel penalty per extra tonne per 1,000 nm | A319 85 kg; A320 82 kg; A321 62 kg; A330-200 49 kg; A330-300 47 kg; A340-300 87 kg; A340-600 65 kg. That is 5-9% of the extra weight per 1,000 nm, or roughly 2-4% per flight hour (my conversion at 430-480 kt, ESTIMATE). It matches the 2.5-4.5% per hour rule in the brief. |
| APU | A320 family 70-125 kg/h; A330/A340 120-210 kg/h; A340-500/600 160-290 kg/h |
| Contingency fuel | 5% of trip fuel, or 3% with an en-route alternate |
| Cost index | CI = cost of time / cost of fuel, in kg/min. A319 on 2,000 nm: CI 0 to CI 20 cuts block time 15 minutes (5%) for 200 kg (2%) more fuel. Very low cost indices save little fuel for the time they cost. |
| Load assumption in the EUROCONTROL tankering study | load factor 80.3%, 124 kg per passenger ([EUROCONTROL think paper 1](https://www.eurocontrol.int/sites/default/files/2020-01/eurocontrol-think-paper-1-fuel-tankering.pdf)) |

---

## 5. Hedging history

### 5.1 Southwest, 1998-2025

Everything in this table is from Southwest's own 10-K filings.

| Item | Fact | Filing |
|---|---|---|
| Instruments | Purchased crude call options until December 1998; fixed-price swaps from December 1998; crude collars from early 2000. By 2008 "a mixture of purchased call options, collar structures, and fixed price swap agreements" on crude oil, heating oil and unleaded gasoline, because jet fuel has no organised futures market | [FY1999](https://www.sec.gov/Archives/edgar/data/0000092380/000095013400002530/0000950134-00-002530.txt), [FY2008](https://sec.gov/Archives/edgar/data/92380/000119312509015591/d10k.htm) |
| 1999 | Swaps covered 77% of Q1 and 74% of Q2; little hedged in the second half because futures stood well above spot; gains US$14.8m | FY1999 |
| 2000 book (at 24 Feb 2000) | 57% Q1, 85% Q2, 100% Q3, 100% Q4; 2001: 50%, 50%, 20% for Q1-Q3 | FY1999 |
| Gains credited to fuel expense | 2002 US$45m; 2003 US$171m; 2004 US$455m; 2005 US$892m; 2006 US$634m; 2007 US$686m (cash settlements US$727m); 2008 US$1,106m (cash settlements almost US$1.3bn). Sum 2002-2008: US$3,989m | [FY2004](https://www.sec.gov/Archives/edgar/data/92380/000095013405002093/d21965e10vk.htm), [FY2006](https://www.sec.gov/Archives/edgar/data/92380/000095013407001724/d42975e10vk.htm), [FY2007](https://www.sec.gov/Archives/edgar/data/92380/000095013408001572/d53331e10vk.htm), FY2008 |
| Book at January 2005 | about 85% of 2005 capped near US$26/bbl crude-equivalent; over 300 derivative contracts for 2005-2009; fair value US$796m | FY2004 |
| Book at January 2006 | over 70% of 2006 near US$36/bbl | [FY2005](https://www.sec.gov/Archives/edgar/data/92380/000095013406001553/d32370e10vk.htm) |
| Book at January 2007 | about 95% of 2007 at about US$50; 65% of 2008 at US$49; over 50% of 2009 at US$51; over 25% of 2010 at US$63; about 15% of 2011 at US$64; 15% of 2012 at US$63 | FY2006 |
| Book at January 2008 | over 70% of 2008 at about US$51; over 55% of 2009 at US$51; nearly 30% of 2010 at US$63; over 15% of 2011 at US$64; over 15% of 2012 at US$63. About 78% of 2008 consumption ended up hedged | FY2007, FY2008 |
| Cost paid per gallon against the industry | Southwest (ex-tax, net of hedges) 2004 US$0.83, 2005 US$1.03, 2006 US$1.53, 2007 US$1.70; US industry average (DOT) US$1.15, 1.65, 1.95, 2.09 | FY2006, FY2007, [BTS](https://www.transtats.bts.gov/fuel.asp) |
| Option premiums expensed | 2006 US$52m; 2007 US$58m; 2008 US$69m, about 2% of the fuel bill | FY2008 |
| The 2008 collapse | Fair value of fuel derivatives swung from assets of US$2.39bn (1,069 current + 1,318 non-current) at end 2007 to a net liability of US$992m at end 2008; unrealised losses in AOCI US$946m after tax; US$240m cash collateral posted to one counterparty where a year earlier Southwest held US$2.0bn from a counterparty; 20 Boeing 737-700s pledged for collateral obligations between US$300m and US$700m | FY2008 |
| Unwind | In Q4 2008 it sold swaps and zero-cost collars to cut the net hedge to about 10% of consumption for each of 2009-2013, locking in a premium over market of 16-17 c/gal for 2009-2011, 10 c in 2012, 8 c in 2013 | FY2008 |
| End of programme | In the second quarter of 2025 the company terminated the hedging contracts it still held, which ran to 2027, and closed its fuel hedging programme, citing "higher fuel hedging premium costs over time". Premium expense was running at US$0.07/gal (Q1 2025 guidance) | [FY2025](https://www.sec.gov/Archives/edgar/data/92380/000009238026000004/luv-20251231.htm), [FY2024](https://www.sec.gov/Archives/edgar/data/92380/000009238025000024/luv-20241231.htm) |

The headline "US$3.5bn saved" is from the press: AP on 30 June 2008 said hedging had saved Southwest about US$3.5bn since 1999, that Q1 2008 hedge gains of US$291m exceeded the quarter's US$34m profit, and that about 70% of 2008 was hedged at US$51 with oil above US$140 ([NBC/AP](https://www.nbcnews.com/id/wbna25419436)). The 10-K gains above sum to US$4.0bn for 2002-2008 including the second half of 2008. Southwest's own history page says US$3.5bn from 1998 to mid-2008, about 83% of its profits in that period ([southwest50.com](https://southwest50.com/our-stories/the-southwest-jet-fuel-hedge-strategy), via search summary; the page would not load directly).

### 5.2 The 2008-09 reversal

Oil fell from US$144 (3 July 2008) to US$34-40 (December 2008). Collars and swaps struck at US$90-130 went deep out of the money and counterparties called margin.

| Airline | Loss or collateral | Source |
|---|---|---|
| United | Total fuel hedge losses "more than US$1.1 billion" in 2008: US$568m non-cash and US$40m cash in fuel expense, US$279m non-cash and US$249m cash in non-operating expense. Cash collateral posted US$965m at 31 Dec 2008. Instruments: purchased calls, collars, three-way and four-way collars | [UAL FY2008 10-K](https://www.sec.gov/Archives/edgar/data/100517/000095013709001460/c49215e10vk.htm) |
| Delta (with Northwest from 30 Oct 2008) | Net hedge loss in fuel cost US$65m in 2008 (gain US$51m in 2007, loss US$108m in 2006); US$1.2bn of hedge margin posted at 31 Dec 2008. Instruments: crude, heating oil and jet fuel swaps, collars and calls | [DAL FY2008 10-K](https://www.sec.gov/Archives/edgar/data/27904/000119312509042726/d10k.htm) |
| American | Hedging reduced fuel expense by US$380m in 2008 (US$239m in 2007, US$97m in 2006), but US$711m of losses sat in AOCI at year end to be expensed over twelve months. About 35% of 2009 hedged with caps averaging US$2.59/gal and collar floors averaging US$1.94/gal | [AMR FY2008 10-K](https://www.sec.gov/Archives/edgar/data/6201/000000620109000009/ar120810k.htm) |
| Southwest | See 5.1: US$992m liability, US$240m collateral, hedge cut to 10% | FY2008 10-K |
| Cathay Pacific | Unrealised mark-to-market losses HK$7.6bn (US$980m) for 2008 inside a record loss of HK$8.56bn; a further HK$1.9bn in January-February 2009; about half of fuel needs hedged out to the end of 2011, break-even on the hedges near US$75 crude | [Wikipedia, Cathay Pacific, citing its results](https://en.wikipedia.org/wiki/Cathay_Pacific); [CAPA](https://centreforaviation.com/analysis/cathay-faces-some-very-difficult-decisions-6126) via search summary |
| Air China | Fair-value losses on fuel hedges RMB 6.8bn (US$994m) at 31 Dec 2008; another report gives RMB 7.5bn | [China Daily](https://covid-19.chinadaily.com.cn/cndy/2009-01/21/content_7415983.htm), [Shanghai Daily](https://archive.shine.cn/business/transport/Hedging-blamed-for-airlines-losses/shdaily.shtml), via search summary |
| China Eastern | About RMB 6.2bn of hedging losses inside a 2008 net loss of RMB 15.3bn (US$2.2bn) | [Washington Times/AP](https://www.washingtontimes.com/news/2009/apr/16/china-eastern-airlines-suffers-22-billion-loss/), via search summary |
| Singapore Airlines | Fuel hedging losses S$543m in the year to March 2009; S$341m in the December 2008 quarter | [SIA Q3 FY08/09 results](https://www.singaporeair.com/saar5/pdf/Investor-Relations/Financial-Results/SGXNET/sgxann-q3fy0809.pdf), [Aviation Strategy](https://aviationstrategy.aero/newsletter/May-2009/3/Tough-times-ahead-for-SIA), via search summary |

Lesson for the model: the damage was collateral and liquidity as much as P&L. Sold puts (the floor leg of a zero-cost collar) are the instrument that hurts in a crash, and the crash arrives together with a demand collapse.

### 5.3 US majors leave

| Carrier | Evidence | Source |
|---|---|---|
| Delta | Airline-segment hedge result by year: 2010 loss US$89m; 2011 gain US$420m; 2012 loss US$66m; 2013 gain US$444m; 2014 loss US$2,258m; 2015 loss US$935m; 2016 loss US$281m; 2017 loss US$33m. Margin posted US$925m at end 2014 and US$119m at end 2015. In 2015 it deferred part of the book into 2016, then terminated some of those deferrals early. By FY2025 "substantially all" fuel derivatives relate to refinery inventory | [FY2012](https://www.sec.gov/Archives/edgar/data/27904/000144530513000209/dal1231201210k.htm), [FY2015](https://www.sec.gov/Archives/edgar/data/27904/000002790416000018/dal1231201510k.htm), [FY2017](https://www.sec.gov/Archives/edgar/data/27904/000002790418000006/dal1231201710k.htm), [FY2025](https://www.sec.gov/Archives/edgar/data/27904/000002790426000013/dal-20251231.htm) 10-Ks |
| American | After the December 2013 merger: "As of December 31, 2015, we did not have any fuel hedging contracts outstanding"; its policy is not to hedge and it will "continue to be fully exposed to fluctuations in fuel prices". Legacy AMR had hedged | [AAL FY2015 10-K](https://www.sec.gov/Archives/edgar/data/6201/000119312516474605/d78287d10k.htm) |
| United | At 31 December 2017 the company had no fuel hedging contracts outstanding; "current strategy is to not enter into transactions to hedge its fuel consumption" | [UAL FY2017 10-K](https://www.sec.gov/Archives/edgar/data/100517/000119312518054235/d471340d10k.htm) |
| Southwest | Closed the programme in Q2 2025 | FY2025 10-K |

Correction to the brief: Delta's roughly US$2.3bn loss is the 2014 figure (the mark-to-market taken as oil fell in late 2014); 2015 added US$935m, so the two years together cost US$3.2bn. All four large US carriers entered the 2026 shock unhedged.

Why they left (my synthesis of the filings and of Morrell and Swan, below): the 2008 and 2014 losses arrived with margin calls; unhedged US competitors with similar cost bases move fares together, so fuel is a pass-through cost for the group; premiums rose with volatility; and consolidation reduced the bankruptcy risk that hedging insures against.

### 5.4 Delta's refinery

| Item | Fact | Source |
|---|---|---|
| Deal | Monroe Energy bought the idled Trainer, Pennsylvania refinery from Phillips 66 in June 2012 for US$180m, less a US$30m Pennsylvania grant (US$150m net); restart September 2012; about US$100m planned to raise jet yield; capacity 185,000 b/d then, "approximately 200,000" now | [DAL FY2012 10-K](https://www.sec.gov/Archives/edgar/data/27904/000144530513000209/dal1231201210k.htm), [View from the Wing, 11 Oct 2026](https://viewfromthewing.com/deltas-oil-refinery-looked-like-a-bad-bet-then-it-made-1-36-billion-but-the-cheap-fuel-story-is-wrong/), [Wikipedia, Trainer Refinery](https://en.wikipedia.org/wiki/Trainer_Refinery) |
| Rationale in Delta's words | "strategy to mitigate the increasing cost of the refining margin we are paying" | FY2012 10-K |
| Structure | BP supplied crude under a three-year agreement; non-jet products are exchanged with Phillips 66 and sold to BP for jet fuel delivered elsewhere on Delta's network; pipelines reach LaGuardia and JFK. July 2014: five-year deal for 65,000 b/d of domestic crude, a third of intake | FY2012 10-K, Wikipedia |
| Promise | US$300m a year of savings; investment recovered "in the first year of operations" | View from the Wing, quoting Delta |
| Segment operating result, US$m | 2012 -63; 2013 -116; 2014 +96; 2015 +290; 2016 -125; 2017 +110; 2018 +58; 2019 +76; 2020 -216; 2021 -2; 2022 +777; 2023 +385; 2024 +38; 2025 +157. First half 2026 +311; Q3 2026 -33 after a fire on 25 June | View from the Wing table compiled from Delta filings. The 2013-2017 values match the fuel-expense tables in the FY2015 and FY2017 10-Ks, which I read directly |
| Totals | US$108m over 2012-2021; US$1,357m over 2022-2025; capital expenditure US$1.62bn through 2025 | View from the Wing |
| Other | 2018: Delta sought a partner to take a stake. The refinery blends little renewable fuel, so it buys most of its RINs | View from the Wing; FY2025 10-K |

What it is: a physical long position in the crack spread, sized to roughly Delta's own jet exposure in the Northeast, which loses money in normal years and pays in exactly the regime where a crude hedge fails (2022-23, 2026). Transfers to the airline are at market price, so it is an offset in the consolidated accounts, not cheaper fuel.

### 5.5 European and Asia-Pacific policies

Positions reported in a Reuters factbox dated 3 March 2026, days after the shock began ([Hydrocarbon Processing reprint](https://hydrocarbonprocessing.com/news/2026/03/how-airlines-have-hedged-against-fuel-price-increases/)), plus company documents.

| Airline | Policy and position |
|---|---|
| Ryanair | Forward swaps (and at times call options) on jet fuel, 12-18 months ahead. 80% of FY2027 (to March 2027) hedged at US$668/t; the books held a year and two years earlier averaged US$761/t and US$795/t. Spot jet went from US$700/t in March 2026 to over US$1,400/t in early May. Also hedges EU carbon allowances ([annual report 2026](https://investor.ryanair.com/wp-content/uploads/2026/06/Ryanair-2026-Annual-Report.pdf), [20-F](https://www.sec.gov/Archives/edgar/data/1038683/000110465926076131/tmb-20260331x20f.htm)) |
| easyJet | 84% of H1 FY2026 at US$715/t, 62% of H2 at US$688/t, 43% of H1 FY2027 at US$671/t; dollars hedged in parallel (80%, 62%, 40%) |
| Wizz Air | January 2026: 83% of the year to March 2026 at US$681-749/t, 55% of FY2027 at US$650-716/t, 7% of FY2028. By 29 July 2026: 76% of FY2027 at US$749-837/t and 39% of H1 FY2028 at US$776-864/t. Price bands indicate collars. Quarter to June 2026: loss EUR 198.6m against a EUR 39.3m profit a year earlier, about EUR 100m of the swing from fuel ([Aviation Business News](https://aviationbusinessnews.com/low-cost/latest-news-low-cost/wide-fuel-margins-keep-pressure-on-lcc-hedge-books)) |
| IAG | Three-year rolling programme; up to 75% of near-term requirements, up to 80% for its low-cost airlines |
| Lufthansa | Horizon up to 24 months; at end 2024 about 76% of 2025 and 28% of 2026 were hedged |
| Air France-KLM | February 2026: raised cover over the next year from 68% to 87% and extended the horizon from six quarters to eight |
| Finnair | Horizon extended from 18 to 24 months in December 2025; target 70-95% for the first three months, declining by quarter; Q1 2026 at US$718/t average |
| SAS | Policy 40-80% of the next 12 months, up to 50% for the following six; actual position 0% |
| Icelandair | 20-50% for months 1-6, 0-40% for months 7-12, 0-20% for months 13-18 |
| Norwegian | About 45% of 2026 and 25% of 2027 |
| Singapore Airlines | Up to five years ahead, in both Brent and MOPS (Singapore jet): 49% of the December 2025 quarter, 47% of the March 2026 quarter, 24% of H2 FY2027, 7% beyond; Brent at US$66-69, MOPS at US$79-87/bbl |
| Cathay Pacific | About 30% covered through Q2 2026, hedging into Q2 2027 |
| Qantas / Virgin Australia / Air New Zealand | 81% / 85% / 83% of the half-year to June 2026; Air New Zealand mostly in Brent with some Singapore jet swaps |
| China Eastern | No jet fuel hedges outstanding at 30 June 2025 |
| US majors | None (5.3) |

IATA's aggregate: airlines had hedged about one-third of expected 2026 consumption ([AFM](https://afm.aero/iata-reports-121percent-global-jet-fuel-price-shock-in-2026-with-india-and-japan-most-exposed)); many are "not in a position to hedge" at all ([Reuters, 3 Jun 2026](https://finance.yahoo.com/sectors/energy/articles/many-airlines-hit-hard-jet-124112122.html)). An unsourced line on Wikipedia's fuel-hedging page puts the industry ratio at 64% in 2009-2010.

Cathay's later episode: HK$4.5bn lost on fuel bets in the first half of 2016 after hedging far forward in 2014 ([Wikipedia, Cathay Pacific](https://en.wikipedia.org/wiki/Cathay_Pacific), citing SCMP).

### 5.6 Instruments

| Instrument | Payoff for the airline | Upfront cost | Risk left |
|---|---|---|---|
| Fixed-price swap (jet, gasoil or crude) | Pays fixed, receives floating monthly average | None; dealer margin and credit charge in the fixed price | Full downside if prices fall; margin calls |
| Purchased call (cap) | Receives max(0, average - strike) | Premium | None beyond the premium |
| Zero-cost collar | Long call at K2, short put at K1 | None | Pays out below K1; margin calls. This was the 2008 instrument |
| Three-way and four-way collars, call spreads | Collar plus a sold higher call or a bought lower put | Low or none | Protection stops above the sold call; United listed both in 2008 |
| Crack swap, jet differential swap, regrade | Fixes jet minus crude or jet minus gasoil | None | Illiquid beyond 12-18 months |
| Crude or heating oil as proxy | Liquid to five years or more | As above | Basis risk, quantified in 2.3 |
| Physical: refinery ownership, tankering, storage | Long crack; location arbitrage | Capital | Operating risk |

Morrell and Swan note that crude and exchange-traded contracts are the most liquid, that over-the-counter jet contracts carry counterparty risk, and that only carriers with cash and credit can hedge at all ([abstract](https://ideas.repec.org/a/taf/transr/v26y2006i6p713-730.html)).

Option cost. Sourced data points: Southwest paid US$52-69m a year in premiums in 2006-2008, about 2% of its fuel bill, for a book covering 70-95% of the next year; US$0.07/gal in 2025, about 3% of a US$2.50 fuel cost, which it judged too much. Black-76 values as a share of notional, zero rates (OWN CALC from the formula, not market quotes):

| Volatility | Strike | 3-month call | 6-month | 12-month | Strip of 12 monthly calls (months 1-12) |
|---|---|---|---|---|---|
| 25% | at the money | 5.0% | 7.0% | 9.9% | 7.0% |
| 25% | +10% | 1.7% | 3.4% | 6.2% | 3.5% |
| 25% | +20% | 0.4% | 1.5% | 3.7% | 1.7% |
| 35% | at the money | 7.0% | 9.8% | 13.9% | 9.8% |
| 35% | +10% | 3.4% | 6.1% | 10.1% | 6.1% |
| 35% | +20% | 1.5% | 3.6% | 7.3% | 3.8% |
| 50% | at the money | 9.9% | 14.0% | 19.7% | 13.9% |
| 50% | +10% | 6.2% | 10.3% | 16.1% | 10.2% |
| 50% | +20% | 3.7% | 7.4% | 13.1% | 7.6% |

A zero-cost collar that buys the +10% call strip is funded by selling a put strip about 8% below the forward (strike 0.92 of forward at all three volatilities). Airline options settle on a monthly average price, which trims these values by a few percent only. Capping a year of fuel 20% above the forward at 35% volatility costs about 3.8% of the hedged notional; at 60% coverage that is 2.3% of the fuel bill, in line with what Southwest reported.

### 5.7 Academic and teaching material

| Work | Finding | Status |
|---|---|---|
| Carter, Rogers and Simkins (2006), "Does Hedging Affect Firm Value? Evidence from the US Airline Industry", Financial Management 35(1), 53-86 | US airlines 1992-2003: jet fuel hedging is positively related to firm value; the hedging premium "may be as large as 10%"; most of it comes through the interaction with capital investment, consistent with reducing underinvestment costs (Froot, Scharfstein and Stein 1993) | [RePEc record](https://ideas.repec.org/a/bla/finmgt/v35y2006i1p53-86.html) read |
| Carter, Rogers and Simkins (2006), Journal of Applied Corporate Finance 18(4), 21-33 | Practitioner version; 28 airlines; hedgers had Tobin's Q 5-10% higher | [RePEc record](https://ideas.repec.org/a/bla/jacrfn/v18y2006i4p21-33.html) via search summary |
| Morrell and Swan (2006), "Airline Jet Fuel Hedging: Theory and Practice", Transport Reviews 26(6), 713-730 | Most international airlines hedge fuel, and the theoretical case for doing so is weak (paraphrased). Hedging protects profit when oil spikes on a supply shock that also slows growth; when oil rises on strong demand, hedge gains add to traffic and yield gains and increase volatility. Residual motives: moving profit between periods, bankruptcy protection, signalling competence | [RePEc record](https://ideas.repec.org/a/taf/transr/v26y2006i6p713-730.html) read |
| Treanor et al. (2014), International Review of Financial Analysis 34, 200-211 | Finds a positive premium, but hedging more when exposure is higher does not add value | [RePEc record](https://econpapers.repec.org/RePEc:eee:finana:v:34:y:2014:i:c:p:200-211) via search summary |
| Carter, Rogers and Simkins, "Fuel Hedging in the Airline Industry: The Case of Southwest Airlines" (teaching case, about 2004) | Students choose among swaps, calls, collars and proxy hedges for Southwest in 2001 | RECALLED, UNVERIFIED |
| Cobbs and Wolf (2004), "Jet Fuel Hedging Strategies: Options Available for Airlines and a Survey of Industry Practices", Kellogg | Instrument primer and survey of US carriers' hedge ratios | RECALLED, UNVERIFIED; the Kellogg link returned an error |
| Harvard Business School cases on Southwest hedging or on Delta's refinery | I could not confirm that a case with either title exists | NOT FOUND |

---

## 6. How airlines respond to fuel price changes

### 6.1 Fuel surcharges

| Topic | Fact | Source |
|---|---|---|
| Nature | A carrier-imposed charge filed outside the base fare (YQ or YR on the ticket). US DOT guidance of 2012: anything described as a fuel surcharge "must actually reflect a reasonable estimate of the per-passenger fuel costs incurred by the air carrier above some baseline" | [Klophaus, WCTR 2013](https://wctrs-society.com/wp-content/uploads/abstracts/rio/general/1230.pdf) |
| British Airways, 2004-2006 | Passenger surcharge on long-haul rose from about US$10 per round trip in 2004 to nearly US$110 in 2006; cargo surcharge changed more than 20 times, from 4 to 72 cents per kg | [US Department of Justice, 1 Aug 2007](https://www.justice.gov/archive/atr/public/press_releases/2007/224928.htm) |
| Price-fixing case | BA pleaded guilty to fixing passenger fuel surcharges with Virgin Atlantic (August 2004 to February 2006) and cargo rates (March 2002 to February 2006): US fine US$300m. Korean Air also US$300m. Virgin Atlantic received conditional leniency, Lufthansa likewise on cargo. With the UK Office of Fair Trading penalty the total was reported as GBP 271m, which implies the OFT's GBP 121.5m | DOJ release; [Wikipedia, Virgin Atlantic, citing BBC](https://en.wikipedia.org/wiki/Virgin_Atlantic). The GBP 121.5m figure itself is RECALLED, UNVERIFIED |
| Cathay Pacific, cargo | US$60m US fine in June 2008 for air cargo price-fixing | [Wikipedia, Cathay Pacific](https://en.wikipedia.org/wiki/Cathay_Pacific) |
| Japan | Surcharges need approval from the transport ministry and follow one scheme for all carriers: the two-month average of Singapore kerosene sets the level; it is announced about six weeks before taking effect and then fixed for two months of ticketing. In the US dollar table of January 2013: no surcharge below US$60/bbl; each US$10 band adds US$31-44 per sector on Japan to North America, Europe, Middle East and Oceania (US$44 at 60-70, US$176 at 90-100, US$327 at 130-140, US$359 at 140-150) and US$3-34 on Japan-Korea. Klophaus shows Lufthansa did not recover 100% of incremental fuel cost under it. Today's table for Japan-issued tickets is in yen per barrel | Klophaus, WCTR 2013; the yen thresholds are RECALLED, UNVERIFIED (ANA and JAL pages timed out or refused) |
| Hong Kong | The Civil Aviation Department approved surcharges on flights from Hong Kong; on 20 January 2016 it announced none would be approved from 1 February 2016; the suspension lasted 22 months before airlines were freed to set their own. Cathay reintroduced HK$146 short-haul and HK$652 long-haul per sector for tickets issued from 2 November 2018, revised monthly | [HK government, 20 Jan 2016](https://www.info.gov.hk/gia/general/201601/20/P201601200585.htm), [21 Sep 2018](https://www.info.gov.hk/gia/general/201809/21/P2018092100522.htm), [Business Traveller](https://www.businesstraveller.com/business-travel/2018/09/28/cathays-flights-out-of-hong-kong-are-about-to-get-more-expensive/), via search summary. The two dates do not make 22 months between them; check the government releases before using the dates |
| Brazil | ANAC prohibited separate fuel surcharges on itineraries originating in Brazil; a later rule change is reported to have lifted the ban | Klophaus, WCTR 2013; [One Mile at a Time](https://onemileatatime.com/community/british-airways/brazil-eliminates-their-ban-on-airline-fuel-surcharges/) (title only seen) |
| European Union | Regulation 1008/2008 article 23 requires the displayed price to include all unavoidable charges, so a surcharge is a labelling choice | Klophaus, WCTR 2013 |
| 2026 | ANA and JAL about US$164 per passenger per long-haul segment; Cathay around US$200 on some long-haul segments, cut by up to 12.9% from 16 May after rises since March; IndiGo up to INR 2,300 on Europe routes; SunExpress EUR 10; Air France-KLM long-haul fares up about EUR 50 per round trip; European carriers mostly fold the increase into base fares; US carriers have not brought back surcharges and raised bag fees (JetBlue and United to US$45-55 on some routes); Brazilian carriers say fares need about +10% per US$1/gal and had raised fares over 20% | [eTurboNews, 7 Apr 2026](https://eturbonews.com/airlines-fuel-surcharges-2026-flight-prices-rise/); [Macau Daily Times](https://macaudailytimes.com.mo/cathay-pacific-to-ease-fuel-surcharges-by-up-to-13-starting-may-16.html) via search summary |

Why carriers like the surcharge instead of a fare rise (Klophaus, and RECALLED practice): it changes every fare in a market with one filing; it is usually excluded from corporate and agency discounts and from commission; it applies to award tickets; and it can be reversed without refiling fares.

### 6.2 Pass-through of fuel cost into fares

| Finding | Source |
|---|---|
| "Research examining US airlines indicates a typical 70 percent pass-through rate"; after three to four months most of a cost increase is in fares; fuel is about 30% of a ticket price, so a doubling of fuel implies fares up 20-25% | [McKinsey, 2026](https://www.mckinsey.com/industries/travel/our-insights/what-could-rising-fuel-costs-mean-for-airlines), via search summary; the page timed out on direct fetch and McKinsey does not name the study |
| Pass-through from crude to fares can be positive or negative by market; it falls with the carrier's hedge ratio and with distance and rises with competition on the route; even where positive, only part of a fuel saving reached passengers in 2014-15 | [Gayle and Lin (2020), MPRA 102018](https://econpapers.repec.org/RePEc:pra:mprapa:102018); published in Economic Inquiry 2021 |
| Pass-through of lagged jet prices into revenue differs with each airline's competitive position; consolidation raised it | [Wolter, Ehlers, Luetjens and Gollnick (2021), J. Air Transport Management 95](https://ideas.repec.org/a/eee/jaitra/v95y2021ics0969699721000831.html) |
| Schedules firm up about three months ahead, which is one reason for the lag | commentary via search summary |
| 2008, American: mainline unit revenue +7.3%, yet fare increases and surcharges "did not keep pace"; fuel expense rose US$2.7bn | [AMR FY2008 10-K](https://www.sec.gov/Archives/edgar/data/6201/000000620109000009/ar120810k.htm) |
| 2008, United: mainline yield +6.9% from "industry capacity reductions and fare increases, including fuel surcharges"; it "was not able to financially compensate for the substantial increase in fuel prices" | [UAL FY2008 10-K](https://www.sec.gov/Archives/edgar/data/100517/000095013709001460/c49215e10vk.htm) |
| 2026, industry: passenger ticket yield forecast +7.0% against jet fuel +69% and fuel share rising from 25.4% to 31.4%; net profit halves from US$45.0bn to US$23.0bn; capacity (ASK) growth 1.6% against 5.2% in 2025 | [IATA industry statistics, June 2026](https://www.iata.org/en/iata-repository/pressroom/fact-sheets/industry-statistics/) |
| 2014-16 in reverse: jet fell from US$114.8 to US$52.1/bbl; passenger yield fell 5.5%, 11.9% and 8.8% in 2014, 2015, 2016 | [IATA mid-2018 table](https://www.iata.org/en/iata-repository/publications/economic-reports/airline-industry-economic-performance---2018-mid-year---table) |

A back-of-envelope check on 2026 (ESTIMATE): fuel at 25% of cost rising 69% lifts unit cost about 17%; yield rises 7%; so roughly 40% of the increase is recovered within the year, more once the lag clears and hedges roll off. In 2015-16 yields fell about 20% cumulatively while jet fell 55% from a 31% share of cost (a 17% cut in unit cost), so more than the whole saving reached fares over two years; capacity growth and a strong dollar both contributed. The asymmetry in the literature is real: decreases pass through via capacity additions, increases via capacity cuts, and both take quarters.

### 6.3 Capacity, fees and fleet in 2008

| Carrier | Response | Source |
|---|---|---|
| United | Mainline domestic capacity -8% for 2008 and -14% in Q4; international +1% for the year and -8% in Q4; consolidated about -4% and -11%. Removed 100 aircraft: all 94 Boeing 737s and six 747s, 51 of them gone by year end. US$15 first checked bag and US$25 second bag on domestic flights; passenger and cargo fuel surcharges | [UAL FY2008 10-K](https://www.sec.gov/Archives/edgar/data/100517/000095013709001460/c49215e10vk.htm) |
| Delta | 2009 plan: consolidated capacity -6 to -8%, domestic -8 to -10%, international -3 to -5% | [DAL FY2008 10-K](https://www.sec.gov/Archives/edgar/data/27904/000119312509042726/d10k.htm) |
| American | Capacity cuts announced in Q2 2008 and extended for 2009; US$1.1bn impairment on the MD-80 and Embraer RJ-135 fleets; US$33m charge for grounding leased A300s; seven MD-80s and one A300 retired in January 2009 | [AMR FY2008 10-K](https://www.sec.gov/Archives/edgar/data/6201/000000620109000009/ar120810k.htm) |
| American, first checked bag fee | Announced 21 May 2008, US$15 from 15 June 2008, together with a Q4 domestic capacity cut of 11-12% and the retirement of at least 75 aircraft | RECALLED, UNVERIFIED (the brief's date matches my recollection; not in the part of the 10-K I could read) |

### 6.4 Tankering

| Item | Number | Source |
|---|---|---|
| Prevalence in Europe (simulation) | Full tankering feasible on 16.5% of flights, partial on 4.5%; pilots and dispatchers report full on 15% and partial on a further 15% | [EUROCONTROL think paper 1, June 2019](https://www.eurocontrol.int/sites/default/files/2020-01/eurocontrol-think-paper-1-fuel-tankering.pdf) |
| Motive | Price in 90% of cases; strikes, fuel shortages, quick turnarounds and fuel quality in 10% | same |
| Burn penalty, narrowbody | 300 nm leg: carrying the return fuel raises trip burn from 2,037 to 2,082 kg (+2.21%, 45 kg). 600 nm leg: 3,592 to 3,760 kg (+4.66%, 168 kg) | same |
| Economics at EUR 550/t and EUR 20/t CO2 | 300 nm: net -EUR 28 at equal prices, +EUR 84 at a 10% price difference, +EUR 196 at 20%, +EUR 308 at 30%. 600 nm: -103, +95, +293, +490 | same |
| Totals for ECAC | 286,000 t of extra fuel burned and 901,000 t of CO2 a year; cost of carrying EUR 157m; net saving to airlines EUR 265m a year | same |
| Decision rule | Tanker if price at destination / price at origin exceeds the transport coefficient K = change in take-off weight per unit of landing weight. Airbus's illustration uses K = 1.3 (1,300 kg loaded leaves 1,000 kg on arrival) | [Airbus, Getting to grips with fuel economy](https://ansperformance.eu/library/airbus-fuel-economy.pdf) |
| Regulation | ReFuelEU article 5: an operator must uplift at least 90% of its yearly fuel requirement at each Union airport; the fine is at least twice the yearly average fuel price times the non-tanked shortfall. Switzerland applies the rule from 1 January 2026 | [Regulation (EU) 2023/2405](https://eur-lex.europa.eu/eli/reg/2023/2405/oj), [European Commission](https://transport.ec.europa.eu/transport-modes/air/environment/refueleu-aviation_en) |

Scope limits in the study: full tankering only below 1,500 nm, partial below 2,500 nm. Landing-weight and tank limits bind before economics do on longer legs.

### 6.5 Fleet and speed

High fuel retires the oldest and thirstiest types first, and the EUROCONTROL data shows why. On a 500 nm stage (OWN CALC, seats as assumed in 4.2) an MD-82 burns 31.7 g per seat-km and a 737-300 30.9, against 23.3 for a 737-800, 20.0 for a MAX 8 and 18.6 for an A320neo. In 2008 that meant United's 737 Classics and some 747s, American's MD-80s and A300s (6.3). The 747-400 at 28 g per seat-km against 20-22 for a 787-9 or A350 explains the 2014-2020 exits. Low fuel does the opposite: it keeps old aircraft flying and delays orders.

Slow flying: lowering the cost index trades minutes for kilograms. On an A319 over 2,000 nm, going from cost index 20 to 0 saves about 200 kg (2%) and costs 15 minutes (Airbus, above). A carrier in a fuel spike can recover 1-2% of burn this way before block times and crew costs push back.

---

## 7. SAF mandates and carbon cost

| Item | Number | Source |
|---|---|---|
| EU allowance price | EUR 82.16 on 18 Nov 2026; record EUR 105.73 in February 2023; 2026 range roughly EUR 80-90 from the data points shown | [Trading Economics](https://tradingeconomics.com/commodity/carbon) |
| History | Below EUR 10 in 2012-2017 (low EUR 2.81 in January 2013); EUR 18-30 from August 2018 to March 2020; EUR 57 in July 2021; EUR 97 in February 2022; above EUR 100 then about EUR 70 in 2023 | [Wikipedia, EU ETS](https://en.wikipedia.org/wiki/European_Union_Emissions_Trading_System) |
| Aviation scope | In the scheme since 2012; limited to flights within the EEA (plus departures to the UK and Switzerland) until the start of 2027; free allocation cut 25% in 2024 and 50% in 2025, full auctioning from 2026 | [European Commission](https://climate.ec.europa.eu/eu-action/transport-decarbonisation/reducing-emissions-aviation_en) |
| Exposure per tonne of fuel | 3.15-3.16 t CO2 x allowance price: EUR 221 at EUR 70, EUR 259 at EUR 82, EUR 316 at EUR 100. Against the 2024 reference fuel price of EUR 734/t that is +30% to +43% on covered flights | arithmetic on the sources above and below |
| Ryanair as a check | EUR 843m of EU ETS cost in FY2026 (EUR 822m in FY2025) inside a fuel and oil line of EUR 5,419m; a EUR 1 change in the allowance price moves its carbon cost by about EUR 12m; it reckons ETS adds EUR 6-10 per flight per passenger | [Ryanair annual report 2026](https://investor.ryanair.com/wp-content/uploads/2026/06/Ryanair-2026-Annual-Report.pdf) |
| SAF support inside the ETS | 20 million allowances (EUR 1.6bn at EUR 80) cover part of the SAF price gap from 2024: 95% for synthetic fuels, 70% for advanced biofuels, 50% for other eligible fuels, up to 100% at small and island airports | European Commission, above |
| ReFuelEU blending mandate | 2% from 2025; 6% from 2030; 20% from 2035; 34% from 2040; 42% from 2045; 70% from 2050. Synthetic sub-mandate: average 1.2% in 2030-31, 2.0% in 2032-34, then 5%, 10%, 15%, 35%. Applies to fuel supplied at EU airports above 800,000 passengers or 100,000 t of freight | [Regulation (EU) 2023/2405, Annex I](https://eur-lex.europa.eu/eli/reg/2023/2405/oj) |
| EU reference prices, 2024 | Conventional jet EUR 734/t; aviation biofuel EUR 2,085/t (2.8 times); advanced biofuel EUR 2,715/t estimated (range 1,915-3,655); recycled-carbon fuel EUR 2,280/t; synthetic e-kerosene EUR 7,695/t estimated (range 6,820-9,405; 10.5 times) | [EASA briefing note, 25 Feb 2025](https://www.easa.europa.eu/en/downloads/141675/en) |
| IATA, 2026 | SAF US$2,872/t (US$374/bbl), about 2.5 times its jet forecast of US$152/bbl; output 1.9 Mt in 2025 and 2.4 Mt in 2026, 0.8% of jet demand; the premium adds US$4.3bn to the industry fuel bill | [IATA fuel fact sheet](https://www.iata.org/en/iata-repository/pressroom/fact-sheets/fact-sheet-fuel/) |
| Airline view | SAF "can be up to four times the cost of normal jet kerosene"; Ryanair sourced a 2% blend at EU airports | Ryanair annual report 2026 |
| Blended cost of the mandate (ESTIMATE from EASA prices) | 2% biofuel: +EUR 27/t (+3.7%). 6% biofuel: +EUR 81/t (+11%). 2030 mix of 4.8% biofuel and 1.2% e-fuel: +EUR 148/t (+20%), before ETS allowances for SAF | arithmetic |

CORSIA applies to international flights outside the ETS scope; its second phase runs 2027-2035 (European Commission, above). I did not retrieve CORSIA unit prices.

---

## 8. Gaps and unverified items

| Item | Status |
|---|---|
| Schwartz (1997) and Schwartz-Smith (2000) parameter tables | Quoted from memory. The UCLA-hosted scan of a Schwartz paper has no text layer and I could not confirm which paper it is |
| Singapore and Rotterdam jet price history | Proprietary. Only three IATA regional snapshots and IATA's quoted hub spreads were found. US regional basis uses diesel as a proxy |
| Into-plane fees | No authoritative schedule found. Section 1.5 gives a measured all-in differential for US carriers and vendor ranges of low quality |
| OFT fine on British Airways (GBP 121.5m) | From memory; consistent with the GBP 271m combined total cited on Wikipedia and the US$300m DOJ fine |
| American's 21 May 2008 bag-fee announcement and the 75-aircraft retirement | From memory |
| Japan's current yen-denominated surcharge table | Not retrieved; the 2013 US dollar table is sourced |
| Hong Kong dates | From a search summary of government releases; internal inconsistency noted in 6.1 |
| Harvard Business School cases | None confirmed |
| Southwest's hedge gains for 2000 and 2001, and premium expense after 2008 | Not retrieved (the FY2024 and FY2025 notes sit beyond what I could read); only the US$0.07/gal figure for Q1 2025 |
| Delta refinery results 2018-2025 | From a secondary compilation; the 2013-2017 values in it match Delta's 10-Ks |
| McKinsey's 70% pass-through | Seen in a search summary; the study behind it is not named |
| Pass-through coefficients in Gayle and Lin, and in Wolter et al. | Abstracts only |
| IATA jet price for 2018; fuel share for 2018 | Only the mid-2018 forecast and an implied bill |
| 2026 data | Year to date. DOT July-September 2026 figures are preliminary |

---

## 9. Model recommendations

### (a) Daily jet price process with regional basis and shock events

Structure: jet price at hub r on day t = Brent + reference crack + regional basis, then a contract and into-plane layer turns the hub price into what the airline pays. The reference hub is the US Gulf Coast, because that is the only long public daily series and it tracks IATA's global average within about US$3/bbl (1.4). Step in trading days, dt = 1/252.

**Crude (Brent, US$/bbl).** Two-factor log model, ln B = L + S.

| Parameter | Default | Basis |
|---|---|---|
| Long-run level exp(mean of L) | US$75/bbl in 2025 dollars; drift with inflation | Fit 2005-2026 gives US$76 (2.2) |
| L: kappa_L, sigma_L | 0.10 per year, 0.14 | Schwartz-Smith long factor about 0.12-0.15 (recalled); slow reversion keeps L inside US$40-140 |
| S: kappa_S, sigma_S | 1.5 per year (half-life 5.5 months), 0.28 | Regime fits 0.8-1.25 per year for the single-factor log price (2.2); Schwartz-Smith short factor 1.2-1.5 and 16-29% (recalled) |
| Correlation of the two shocks | 0.3 | Schwartz-Smith (recalled) |
| Jumps on S | Poisson 6 per year, size normal(0, 0.07) | 2.1: 5.6 clipped days a year with sd 6.5% in a calm decade, 8.7 with sd 10.6% over the full sample |
| Resulting volatility | about 39% of daily returns annualised; unconditional 1-sd band about US$50-110 | Observed 30% calm, 44% full sample |

A one-factor fallback: d ln B = 0.5 (ln 75 - ln B) dt + 0.33 dW, with the same jumps.

**Reference crack, US Gulf Coast jet minus Brent (US$/bbl), C = m(B) + slow + fast + regime.**

| Parameter | Default | Basis |
|---|---|---|
| Mean m(B) | max(6, 0.19 x B) | Jet/Brent median 1.18-1.19 in 2004-2019 |
| Slow component | OU, half-life 6 months, stationary sd US$4 | Annual averages ranged 8.9-16.8 in 2010-2019 |
| Fast component | OU, half-life 10 trading days, stationary sd US$3.5 (daily innovation about US$1.3) | Fit 2006-19: half-life 13 days, daily sd US$1.9 (2.2). Smaller here because the hub-outage event below supplies the spikes |
| Floor | US$2 | Minimum observed -3.0 for a few days in 2020 |

**Shock events.** Draw each as a Poisson arrival; let effects decay exponentially.

| Event | Rate | Effect | Calibration |
|---|---|---|---|
| Supply or geopolitical shock | 0.08 per year | S jumps +0.35 to +0.55 (Brent +40% to +70%) over 20-40 trading days; crack regime +US$25-45 with half-life 6 months; regional basis volatility x3 and Europe, Africa and Latin America basis +US$5-12 | 1990 (jet +124% in three months), 2022 (jet +68%, crack 40.8 average), 2026 (jet +63% in a month, crack 52 average, peaks 80) |
| Refining or logistics crisis without a crude shock | 0.05 per year | Crack regime +US$20-35, half-life 4 months | 2005 hurricanes, late 2023 |
| Demand collapse | 0.08 per year | S falls 0.8 to 1.1 (Brent -55% to -67%) over 40-120 days; crack compresses to US$3-8 for 12-18 months; air travel demand shock at the same time | 2008-09 (-65%, crack 8.2 in 2009), 2020 (-66%, crack 4.2), 2014-16 glut (-68% over 19 months, crack unaffected) |
| Hub outage (hurricane, refinery or pipeline failure) | 0.3 per year per hub, seasonal for the Gulf Coast (August-October) | That hub's basis +30 to +150 c/gal, half-life 5-10 trading days | Gulf Coast crack 72 (Oct 2005) and 108 (Sep 2008); New York Harbor +US$3/gal in April 2022 |

Rates are ESTIMATE from four crack-regime episodes and six large crude moves in 36 years. With these rates about 15% of simulated years should contain a month with the crack above US$35; the record has five such years in 36 (2005, 2008, 2022, 2023, 2026).

**Regional basis (added to Brent + crack; c/gal; 1 US$/bbl = 2.38 c/gal).** Each is an OU with half-life 10 trading days.

| Hub or region | Calm mean | Crisis mean | Daily innovation sd, calm | Basis |
|---|---|---|---|---|
| US Gulf Coast (reference) | 0 | 0 | 1.5 | Definition |
| New York Harbor | +5 | +10, tail to +140 | 1.6 (5.6 in crisis) | Diesel proxy, 1.4 |
| Chicago and US mid-continent | +3 | +8 | 2.0 | ESTIMATE |
| Los Angeles and US West Coast | +7 | +12 to +26 | 2.3 (7-10 in crisis) | Diesel proxy, 1.4 |
| North West Europe (ARA) | +10 | +12 to +36 | 2.0 | IATA Europe index against Gulf Coast: +11 calm, +12 and +36 crisis (1.4) |
| Mediterranean | +12 | +25 | 2.5 | ESTIMATE |
| Singapore and East Asia | -12 | -21 | 2.0 | IATA Asia & Oceania against Gulf Coast: -12 calm, -21 crisis |
| Middle East Gulf | -22 | -31 (if exports can leave) | 2.0 | IATA Middle East against Gulf Coast: -22 calm, -31 crisis |
| Latin America | +16 | +19 | 3.0 | IATA Latin America against Gulf Coast |
| Africa | +2 to +15 | +20 | 3.0 | IATA Africa +2 at index level in the calm week; inland supply adds cost (ESTIMATE) |

The stationary sd of a basis with half-life 10 days and innovation 2 c/gal is about 5.5 c/gal. Regional means outside the US rest on one calm week and one or two crisis weeks and are ESTIMATE; only the US pairs rest on long series. IATA's own statement that the spread across Singapore, North West Europe, US West Coast and US Gulf Coast ranged from US$2 to US$41/bbl (5-98 c/gal) in a calm nine months is the check: the calm means above span 32 c/gal between Singapore and North West Europe.

**From hub price to price paid.**

| Layer | Default | Basis |
|---|---|---|
| Index averaging | Paid index = 0.5 x this month's hub average + 0.5 x last month's (or a prior-week average for carriers on weekly pricing) | Fit to DOT 2026 monthly data, 1.5 |
| Differential plus into-plane | Mega-hub on a pipeline 6-10 c/gal; large hub 10-15; medium 15-25; small 25-45; island or remote 50-120 | ESTIMATE anchored on the US all-in average of 10-35 c/gal over Gulf Coast spot (which already contains the New York and West Coast basis) and the EUROCONTROL 20-30% airport spreads |
| Tax | US 4.4 c/gal federal; elsewhere international uplift is generally untaxed, domestic uplift often taxed (configure per country) | A4A; ESTIMATE |
| Volume discount | -1 to -3 c/gal for the largest buyer at a station | ESTIMATE |
| Carbon and SAF | Section 7 | |

Validation targets for the generator: annualised volatility of the monthly-average jet price 30-40%; 12-month change with 5th-95th percentiles near -44% and +84%; calm-state jet/Brent ratio 1.10-1.28; quarterly R-squared of jet on Brent near 0.8.

### (b) Fuel burn per type

Use the table in 4.2: trip fuel (kg) = LTO + a + b x d + c x d^2 with d in km, quadratic set where given, linear otherwise.

| Adjustment | Default | Basis |
|---|---|---|
| Route extension | d = great-circle x 1.05 short-haul, x 1.03 long-haul, plus winds | ESTIMATE |
| Taxi | LTO assumes 26 minutes. Scale: subtract 26 x rate, add actual minutes x rate, with rate 10 kg/min (A319, 737-700, A220), 11.5 (A320, 737-800), 13.5 (A321), 18 (757), 25 (A330, 787, 767), 30 (777, A350), 35-45 (747, A380), 3 (turboprops), 6 (E175, CRJ900) | Airbus figures for its types; others ESTIMATE by engine size |
| Payload | The database assumes typical loads. Per tonne of payload change: 5-9% of that weight per 1,000 nm, i.e. about 3% per flight hour | Airbus table, 4.4 |
| Seat density | Per-seat burn scales with 1/seats | |
| Fleet age and engine wear | +2 to +4% for aircraft late in an engine's on-wing life | ESTIMATE |
| Holding and diversion | 2-3 t/h narrowbody, 5-8 t/h widebody in the hold | From cruise kg/h column, reduced for holding speed (ESTIMATE) |
| APU on stand | 100-125 kg/h narrowbody, 175-290 kg/h widebody | Airbus |
| Cost index | Fuel -2% and block time +5% at minimum cost index; fuel +2 to +4% at high cost index | Airbus A319 example; upper figure ESTIMATE |

Block-hour shortcut if the simulator works in hours: use the kg per block-hour column (about 650 for an ATR 72, 1,350 for an E175, 1,900-2,300 for the A320 and 737 families, 3,400 for a 757, 4,600-5,300 for 767, 787 and A330, 5,900-6,600 for the A350, 8,000 for a 777-300ER, 9,500-10,200 for the 747, 12,600 for the A380), remembering that short sectors run higher per hour.

### (c) Hedging instruments a simulated carrier could buy

| Instrument | Terms offered | Cost | Constraints |
|---|---|---|---|
| Brent swap | Monthly-average settlement, 1-36 months ahead | Fixed price = forward + 0.5-1.0% dealer margin | Counts against a credit line; margin call when mark-to-market loss exceeds a threshold |
| Jet swap at a hub (USGC, NWE, Singapore) | 1-18 months, thin to 24 | Forward + 1.0-2.0% | Same |
| Crack swap (jet minus Brent) | 1-12 months | Forward crack + US$0.5-1.5/bbl | Same; the forward crack reverts toward the mean, so hedging after a shock locks in most of it |
| Brent or jet call (cap), monthly average-price | Strike at the money to +30%, 3-24 months | Premium by Black-76 per monthly settlement with implied volatility = model volatility + 3 points. Guide for a 12-month strip at 35% volatility: at the money 9.8% of notional; +10% strike 6.1%; +20% strike 3.8% (table in 5.6) | Paid upfront; no margin |
| Zero-cost collar | Buy call at +10%, sell put at about -8% (solve for zero cost) | None | Margin on the sold put |
| Call spread, three-way | Buy call, sell a higher call (and a put) | 2-4% of notional or none | Protection capped |
| Physical | Tankering (6.4); storage where available; a refinery stake as in 5.4 | Capital | |

Margin and liquidity: require collateral once the mark-to-market liability exceeds an unsecured threshold. Calibrate on 2008: Southwest posted US$240m and pledged 20 aircraft against a US$992m liability; United posted US$965m; Delta US$1.2bn. A default threshold of 1-2% of annual revenue with cash posted above it reproduces those magnitudes (ESTIMATE). Dealers stop quoting to carriers below a liquidity floor, which matches IATA's remark that many airlines cannot hedge.

Forward curve: for pricing swaps use E[price] under the model, pulled toward the long-run level at the model's reversion speed; add a risk premium of zero by default. After a supply shock the curve is backwardated, so swaps look cheap relative to spot and lock in a high absolute level.

Policy archetypes for AI carriers:

| Archetype | Hedge ratio by horizon | Instruments |
|---|---|---|
| European low-cost | 75-90% of months 1-12, 40-60% of months 13-18, 0-10% beyond | Jet swaps, some caps |
| European network | 60-80% of the next quarter declining to 20-30% at 18-24 months, built in monthly layers; up to 36 months for the most conservative | Brent collars and swaps plus short crack swaps |
| Asia-Pacific network | 30-50% of months 1-12, 10-25% of months 13-24, small tail to five years | Brent and Singapore jet swaps |
| Australasian | 80-85% of the current half-year, 40-50% of the next | Brent, some jet |
| US network or low-cost, post-2016 | 0% | None; one carrier owns a refinery |
| Weak-credit carrier | 0% | Cannot obtain lines |
| Southwest 2004-2008 | 70-95% of year 1, 55-65% of year 2, 25-50% of year 3, 15% of years 4-5 | Calls and collars on crude, heating oil and gasoline |

### (d) Surcharge and pass-through behaviour

| Behaviour | Default | Basis |
|---|---|---|
| Long-run pass-through of a fuel cost change into average fare | 70% (range 50-100%) | McKinsey; Gayle and Lin for the direction of the modifiers |
| Modifiers | +15 points where every carrier on the route is unhedged and capacity is tight; -20 points for a carrier that is more than 60% hedged (it has no cost push and may undercut); -10 points on long-haul; +10 points on routes with three or more competitors; near 100% on monopoly leisure routes only if demand is inelastic | Gayle and Lin (signs), ESTIMATE (sizes) |
| Speed | First-order lag with a time constant of 3 months: about 30% of the eventual effect after one month, 65% after three, 85% after six | McKinsey's 3-4 months; schedules fixed about 3 months out |
| Asymmetry | Increases pass through faster than decreases for one to two quarters; over two years decreases pass through more than fully because capacity grows | 2008 filings; IATA yields 2014-16 |
| Surcharge schedule, formula type (Japan style) | Zero below a baseline (US$60/bbl in the 2013 table; raise to US$80-90 for a 2020s start). Per US$10/bbl step: long-haul +US$35-45 per sector, medium-haul +US$15-20, short-haul +US$3-7. Reference = two-month average hub price; announce 6 weeks ahead; hold for two months | Klophaus |
| Surcharge schedule, discretionary type (Hong Kong style after 2018, most carriers) | Monthly revision; long-haul about 4.5 times short-haul (HK$652 against HK$146); about US$165-200 per long-haul sector at jet near US$190/bbl | HK releases; eTurboNews 2026 |
| Where surcharges are not used | US domestic and most European short-haul: raise base fares and fees (bags US$15 in 2008, US$45-55 in 2026) | 10-Ks; eTurboNews |
| Regulator options | Ban separate surcharges (Brazil until the rule changed); require approval (Japan; Hong Kong to 2018); freeze them (Hong Kong 2016-2018); require all-in display (EU, US) | 6.1 |
| Capacity response to a sustained rise of 50% or more | Within two quarters cut 4-11% of system capacity and 8-14% of domestic; ground the oldest 5-15% of the fleet, the highest g/seat-km types first | United, Delta, American 2008 |
| Capacity response to a sustained fall | Capacity growth 2-3 points above trend for two years; retirements deferred | IATA 2015-17: ASK growth with yields falling 5-12% a year |
| Tankering | Tanker when destination price / origin price exceeds 1 + 0.03 x flight hours (narrowbody) or 1 + 0.025 x flight hours (widebody), subject to landing weight; in the EU cap at 10% of annual uplift per airport | Airbus K; EUROCONTROL; ReFuelEU article 5 |
| Slow flying | When fuel exceeds a threshold share of cost (say 30%), lower the cost index: fuel -1 to -2%, block time +2 to +5% | Airbus |
| Demand effect | Apply the model's own fare elasticity to the passed-through fare change; in a supply shock also apply a GDP shock | Morrell and Swan on the two kinds of oil shock |
