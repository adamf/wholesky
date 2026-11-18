# Sources of the reference data

This file names where each data file in `data/` comes from and when it
was read. Each number in a compiled table has a basis letter:

| Letter | Meaning |
| --- | --- |
| S | Read from the named source. |
| C | Computed from sourced data. |
| P | Read from a public reproduction of an industry table. |
| X | Taken from a worked example in an IATA document. |
| R | Recalled from published material and not checked again. |
| E | An estimate. |

Check an R or E value before you quote it.

## Public data sets, as their publishers serve them

| File | Source | Read |
| --- | --- | --- |
| `jet-fuel-gulf-coast-daily.csv` | US Energy Information Administration, series `EER_EPJK_PF4_RGC_DPG`: U.S. Gulf Coast kerosene-type jet fuel spot price FOB, dollars per gallon, daily from 2 April 1990. `https://www.eia.gov/dnav/pet/hist/EER_EPJK_PF4_RGC_DPGD.htm` | 18 November 2026 |
| `brent-monthly.csv` | US Energy Information Administration, series `RBRTE`: Europe Brent spot price FOB, dollars per barrel, monthly from May 1987. | 18 November 2026 |
| `airfare-city-pairs.csv` | US Department of Transportation, Consumer Airfare Report, Table 1: the 1,000 largest city-pair markets of the contiguous states. Passengers a day each way, average one-way fare, the largest carrier and the lowest-fare carrier with their shares and fares. Quarters 2025 Q2 to 2026 Q1. `https://data.transportation.gov/resource/4f3n-jbg2` | 18 November 2026 |
| `airfare-airport-pairs.csv` | The same report, Table 1a: the airport pairs within those markets. `https://data.transportation.gov/resource/tfrh-tu9e` | 18 November 2026 |

Both agencies publish these as public-domain government data. The files
are the published values with the columns renamed. No row is changed.

## Compiled tables

### `aircraft.csv`

| Columns | Source | Basis |
| --- | --- | --- |
| Fuel: `lto_fuel_kg`, `burn_fixed_kg`, `burn_kg_per_km` | EUROCONTROL's master emissions calculator, distributed with the EMEP/EEA air pollutant emission inventory guidebook 2023, chapter 1.A.3.a. It gives the fuel for a landing and take-off cycle and for climb, cruise and descent at stage lengths from 125 nm to the type's range. The fixed and per-kilometre terms are a straight line fitted to those values. | C |
| `mtow_tonnes`, `seats` | Manufacturers' type data. Seats are a typical layout. | R |
| `value_new_musd`, `lease_new_kusd_month`, `lease_10y_kusd_month` | Appraisers' values and lease rates as trade press reproduced them: AVAC through Aviation Strategy (2015, 2019, 2020), IBA (September 2024), Acumen (2026). The `basis` column of each row says which of the three is S and which is E. Appraisers differ by 5 to 15 per cent. | S or E by row |
| `maintenance_usd_block_hour` | Scaled from IATA's Maintenance Cost Executive Commentary for FY2024 (1,522 dollars a flight hour across the fleet) and older Aircraft Commerce figures by type. | E |
| `turn_full_min`, `turn_minimum_min` | The terminal servicing charts in Airbus's and Boeing's Airplane Characteristics for Airport Planning documents and Embraer's airport planning manuals. | S or E by row |

### `airports.csv`

| Columns | Source | Basis |
| --- | --- | --- |
| `slot_tier` | The IATA Worldwide Airport Slot Guidelines list of coordinated airports, recalled, with a tier estimated from recorded slot trades: Heathrow pairs at 60 to 75 million dollars, Washington National at 5 to 6 million, LaGuardia at 2 to 4 million, Newark at 1.2 million. | E |
| `charges_sdr_per_passenger` | Jacobs, Review of Airport Charges 2024: the charges for a landing and a take-off divided by passengers, at 50 airports, as published in a note on the UK Civil Aviation Authority's site in 2025. | S |
| `mct_*` | Station standard minimum connecting times. P rows come from Ethiopian Airlines' public reproduction of the industry table and from carriers' transit pages. X rows are in IATA's SSIM chapter 8. The file that reservation systems use is OAG's or Cirium's, and it is licensed. | by row |

### `alliances.csv`

Members of Star Alliance, oneworld and SkyTeam on 18 November 2026, from
the alliances' member lists as Wikipedia reproduces them. Basis S.

## Numbers in the code

Each package that uses a constant from research says where it comes from
in its comments. The main ones are:

| Package | Number | Source |
| --- | --- | --- |
| `internal/fuel` | Mean reversion and volatility of the price process | Fitted to the EIA series above. A test fits them again from the embedded data. |
| `internal/fuel` | Regional differentials | IATA's weekly jet fuel price monitor for 3 weeks of 2026, against the Gulf Coast price for the same weeks. Estimates outside the United States. |
| `internal/market` | Willingness to pay and the disutility of each fare restriction | The passenger choice model of the MIT PODS simulator, from Peter Belobaba's lectures for MIT course 16.75J (2006). |
| `internal/market` | Price elasticity by region | InterVISTAS for IATA, Estimating Air Travel Demand Elasticities (2007). |
| `internal/pricing` | How fast and how far a carrier matches a fare change | Borenstein's account of the ATPCO case, FareCompare's counts of attempted fare increases 2007 to 2014, and the court records of United States v. AMR and Spirit v. Northwest. |
| `internal/distribution` | Booking fees and commissions | US court findings in US Airways v. Sabre and United States v. Sabre, GAO-03-749, Amadeus and Sabre annual reports, IATA's payment figures for 2025. |
| `internal/dayplan` | Notice a flight needs to take an earlier slot | EUROCONTROL's ATFCM Users Manual. |
| `internal/airport` | En-route charge formula and unit rates | EUROCONTROL's Central Route Charges Office. |
