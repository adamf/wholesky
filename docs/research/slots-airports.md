# Airport slots, ATFM slots, and airport / navigation charges

Research notes for parameterising a world airline simulator. Compiled 18 November 2026.

## How to read this file

Two different things are both called "slots" and are kept apart throughout:

- **(A) AIRPORT SLOT** — a seasonal planning permission to use the full range of airport infrastructure at a coordinated (Level 3) airport on a given date and time, allocated by a coordinator under the IATA/ACI/WWACG Worldwide Airport Slot Guidelines (WASG) and local law. Held for a season, renewable through historic precedence ("grandfather rights"). It is not a route right, not a traffic right, not a gate, and not an ATC clearance.
- **(B) ATFM SLOT** — a day-of-operation time issued by the flow manager: in Europe a Calculated Take-Off Time (CTOT) from EUROCONTROL's CASA; in the US an arrival slot / EDCT under a Ground Delay Program. It exists only while a regulation or program is in force, has no monetary price and no historic rights.

A flight at Heathrow on a thunderstorm day needs both: its airport slot (held since last season) and, separately, a CTOT.

**Evidence tags.** Every number carries one of:

- **[V]** verified in this session against the cited source (fetched or returned by search).
- **[R]** recalled from background knowledge and not re-verified in this session. The session's web-search budget ran out part-way through the charges section, so some items could only be checked by fetching known URLs. Treat [R] items as "probably right, check before hard-coding".
- **ESTIMATE** my own estimate or derivation; the arithmetic is shown where it matters.

Currency conversions used for derived figures: 1 SDR = GBP 1.0395 [V, Jacobs 2024 report] and about USD 1.33 [R].

---

## 1. Airport slot rules

### 1.1 Levels and how many airports are coordinated

| Level | Name | Meaning |
|---|---|---|
| 1 | Non-coordinated | Capacity adequate; no slot process. |
| 2 | Schedules facilitated | Potential congestion in some periods; airlines submit schedules to a facilitator who seeks voluntary adjustments. No historic precedence and no series concept. [V, WASG 4] |
| 3 | Coordinated | Demand significantly exceeds capacity; every operator must hold a slot allocated by an independent coordinator in order to land or take off. [V, WASG 4] |

Counts:

| Source | Level 3 | Level 2 | Passenger share |
|---|---|---|---|
| IATA Airport Slots Fact Sheet, June 2026 | 216 | 179 (derived: 395 airports "subject to coordination" in total) | "around 43% of passengers globally depart from a slot coordinated airport" [V] |
| IATA policy page (2026) | "200+" | "160+" | "50% of all passengers depart from a Level 3 airport"; "35% of all flights operate between Level 3 airports" [V] — the 50% and 43% figures are both IATA's and are not reconciled by IATA |
| ACI EUROPE position paper (March 2025) | 218 worldwide for summer 2025, of which 113 in Europe [V] | | |
| Summer 2022 (via Wikipedia, citing IATA) | 195 | 156 | [V] |

IATA says about 9 airports a year are newly declared Level 2 or Level 3 [V]. Changes of level must be notified by 1 April for the next winter season and 1 September for the next summer season [V].

Regional split of the ~216 Level 3 airports: Europe ~113 [V]; the remainder is my **ESTIMATE**: Asia-Pacific ~60 (China ~21 "main coordinated" airports plus others, India, Japan, South-East Asia, Australia/NZ), Middle East and Africa ~20, Americas ~20-25 (the US has very few: see 1.6). The authoritative list is WASG Annex 12.7, updated each season.

Heathrow operates at about 99% of declared capacity [V, IATA]; over 99% of its summer 2022 weekly slots were held under historic rights [V, IAG paper citing DfT]; British Airways holds over half [V].

### 1.2 The scheduling calendar

Seasons: Northern Summer starts the last Sunday of March, Northern Winter the last Sunday of October [V, WASG 4]. Two IATA Slot Conferences a year (June for winter, November for summer).

Calendar published by IATA (November 2026 update) [V]:

| Activity | NS27 (SC159) | NW27/28 (SC160) | NS28 (SC161) |
|---|---|---|---|
| SHL deadline (coordinator tells each airline its historic slots) | 14 Sep 2026 | 19 Apr 2027 | 13 Sep 2027 |
| Agreed Historics Deadline | 1 Oct 2026 | 6 May 2027 | 30 Sep 2027 |
| Coordination parameters / capacity confirmed no later than | 1 Oct 2026 | 6 May 2027 | 30 Sep 2027 |
| Initial Submission Deadline (airline requests, 23:59 UTC) | 8 Oct 2026 | 13 May 2027 | 7 Oct 2027 |
| SAL deadline (coordinator returns Slot initial Allocation List) | 5 Nov 2026 | 3 Jun 2027 | 4 Nov 2027 |
| IATA Slot Conference | 17-19 Nov 2026 | 15-17 Jun 2027 | 16-18 Nov 2027 |
| Series Return Deadline | 15 Dec 2026 (trial) | 15 Jul 2027 (trial) | 15 Dec 2027 |
| Historics Baseline Date (HBD), 23:59 UTC | 31 Jan 2027 | 31 Aug 2027 | 31 Jan 2028 |
| Start of season | 28 Mar 2027 | 31 Oct 2027 | 26 Mar 2028 |

Rules of thumb for a simulator:

- SHL deadline = third Monday of September (summer) / third Monday of April (winter) [V]. Agreed Historics Deadline = Initial Submission Deadline minus 7 days [V]. Initial Submission is about 5-6 weeks before the conference; SAL about 2 weeks before it.
- Series Return Deadline was 15 January (summer) / 15 August (winter) until now [R]; IATA is trialling 15 December / 15 July for NS27 and NW27/28 [V]. Airlines that deliberately return series after this date get lower priority in the next equivalent season and are named on a published list [V].
- HBD: 31 January (summer) / 31 August (winter) [V]. The series held at HBD is the denominator of the 80% test.

### 1.3 Series, historic precedence and the 80/20 rule

All from WASG Edition 4 (effective 1 August 2025) [V] unless tagged otherwise.

- **Series of slots**: at least 5 slots allocated for the same or approximately the same time on the same day of the week, distributed regularly in the same season. **From NS27**: at least 5 for a winter season and **at least 7 for a summer season**. Each day of week is a separate series; a daily flight is seven series.
- **Historic precedence**: an airline is entitled to the same series in the next equivalent season if it operated it at least 80% of the time during the period allocated. Historic slots may not be confiscated to accommodate new entrants; only proven intentional misuse justifies withdrawal.
- **Denominator**: the series held at HBD. A series newly allocated after HBD is measured from first allocation. Ad hoc slots earn no historic rights (but slots requested as a series and initially allocated ad hoc that form a series by season end may qualify).
- **Cancellations before HBD**: cancelling 5 or more consecutive weeks shortens the period eligible for historics (or splits it); cancelling fewer than 5 consecutive weeks does not, provided total cancellations are 20% or less of the period. Airlines are told to make public-holiday and other planned cancellations on or before HBD so they do not count.
- **Cancellations after HBD**: every one counts as non-use in the 80% calculation unless justified.
- **Justified non-utilisation (JNUS)**: unused slots are treated as operated if non-use was due to (a) interruption by unforeseeable and unavoidable causes outside the airline's control (airport or airspace closure, severe weather), or (b) action intended to affect the services, such as industrial action. The airline should contact the coordinator promptly to have it confirmed. EU law (Reg. 95/93 Art. 10(4)) has a similar closed list, widened in 2022 to cover epidemics, natural disasters and widespread political unrest [R].
- **What a single cancellation does**:
  - The series is still held for the rest of the season and is still eligible for next season if usage stays at or above 80%.
  - The slot for that specific day is simply not used. It cannot be banked, carried forward or moved to another day.
  - The airline is obliged to hand it back immediately ("even at short notice it may be possible to reallocate"). The coordinator can re-allocate it ad hoc to anyone; the ad hoc user gains no historic right (WASG 8.17).
  - The holder may itself use the slot time for a different flight: ad hoc changes of flight number, route, aircraft type or service type do not affect historic eligibility, they only need coordinator confirmation (WASG 8.7.1(i)). A slot is airport-and-time specific, not route specific.
- **Worked arithmetic (derived)**: NS27 runs 28 Mar - 30 Oct 2027 = 31 weeks, so a full-season weekly series has 31 slots; 80% requires 25 operations, leaving 6 unjustified cancellations per day-of-week series. NW27/28 runs 21 weeks: 17 required, 4 spare.
- **Monitoring**: coordinators are expected to give airlines in-season warnings of series at risk; at EU/UK airports coordinators can also fine for slot misuse (operating without a slot, off-slot operations) [R].

### 1.4 The pool, new entrants, swaps, transfers, leases

- Allocation order [V]: unchanged historics first; then a pool is formed (unused, returned and newly created slots); **50% of the pool at initial allocation must go to new-entrant requests** (unless there are fewer requests) and 50% to non-new-entrants; imbalances are corrected over following equivalent seasons. Year-round extensions of existing operations rank above wholly new requests within each category. Additional criteria (connectivity, competition, curfews, market needs, environment) break ties [R].
- **New entrant (WASG 4)** [V]: an airline requesting a series on a day where, if accepted, it would hold fewer than 7 slots at that airport on that day (3 rotations). A new entrant offered slots within 1 hour of its request that refuses loses the status for the season. Under EU Reg. 95/93 (as amended by Reg. 793/2004) the test is fewer than 5 slots that day, or fewer than 5 on an intra-EU route with at most two incumbents, and not more than 5% of the airport's slots [R]. The UK aligned its definition with WASG by SI 2025/158, in force 13 February 2025 [V].
- **Swaps** [V]: one-for-one, between any number of airlines, encouraged. The coordinator may refuse a swap of newly allocated slots if it does not improve the operating position.
- **Transfers** [V]: allowed only where national law permits; only to an airline serving or planning to serve the airport; newly allocated slots may not be transferred until operated for two equivalent seasons (anti-flipping rule). Names, times and period of any swap or transfer for consideration are published by the coordinator; price is not.
- **Shared operations / leases / "babysitting"** [V for the rule]: slots held by one airline may be used by another; the original holder keeps the historic precedence and gets the slots back at the end or if the operator loses its licence. This is the legal form of a slot lease and of alliance partners "babysitting" slots.
- **Secondary trading for money**:
  - UK: lawful since *R v Airport Co-ordination Ltd, ex parte States of Guernsey Transport Board* [1999] EuLR 745, which held that an exchange under Art. 8(4) of Reg. 95/93 is still an exchange when money accompanies it. This produced the "artificial exchange": a valuable slot is swapped for a worthless (often late-night) slot plus cash, and the junk slot is returned to the pool [R].
  - EU: Commission Communication COM(2008) 227 (30 April 2008) said the Commission would not pursue Member States where exchanges for consideration take place transparently [R]. In practice cash trading is concentrated in London (Heathrow, some Gatwick); at Frankfurt, Paris CDG, Madrid, Amsterdam, Munich and Rome airlines swap but do not trade for cash [V, IAG paper to the CAA, May 2026].
  - US: the "buy-sell rule" (14 CFR 93.221), adopted December 1985 (50 FR 52195) [V], effective 1 April 1986 [R], let carriers buy, sell, lease and mortgage High Density Rule slots. FAA rules state that slots are an operating privilege, not property (14 CFR 93.223) [R].
  - ACL (UK coordinator) says it has facilitated more than 200 Heathrow slot trades involving 2,500 weekly slots since 2001, over 25% of all Heathrow slots [V]. Heathrow slot transfers rose from 42 in 2000 to 526 in 2012 [V, via Wikipedia].
- UK reform: DfT consultation "Airport slot allocation system reform" ran December 2023 - 8 March 2024; the outcome was still unpublished when checked [V]. Themes consulted on included new-entrant rules, limits on long-term leasing, trade transparency and coordinator powers [R]. The CMA response argued for market-based primary allocation [V].

### 1.5 Slot waivers and the ghost-flight row

| Episode | Relief |
|---|---|
| 9/11 | EU waiver for summer 2001 / winter 2001-02 (Reg. 894/2002) [R] |
| Iraq war / SARS 2003 | EU waiver (Reg. 1554/2003) [R] |
| Financial crisis | One season only: summer 2009 (Reg. 545/2009) [R] |
| COVID, EU | Full waiver from 1 March 2020 (Reg. 2020/459), extended through winter 2020/21 [R]; summer 2021: 50% threshold with the option to hand back full series before the season (Reg. 2021/250) [R]; winter 2021/22: 50% [V, press reports]; summer 2022: 64% (Delegated Reg. 2022/255) [V]; winter 2022/23 onward: back to 80% with a wider JNUS list (Reg. 2022/2038, signed 19 October 2022; the Council had sought 75%) [V/R] |
| COVID, UK | Full waiver through summer 2021 [R]; winter 2021/22 50:50 [R]; summer 2022 70:30 [V]; winter 2022/23 70:30 retained [V]; summer 2023 back to 80:20 with a limited pre-season hand-back allowance [R] |
| COVID, US | Usage waived at JFK, LGA, DCA from March 2020; phased back from late 2021 [R] |
| New York ATC staffing | Since summer 2023 carriers may return up to 10% of slots at JFK and LGA (and DCA slots used for New York flights) and have them treated as operated, because of controller shortages at the N90 TRACON. Extended repeatedly: to 24 Oct 2026 (FR 25 July 2025) and then through 30 Oct 2027 (FR June 2026) [V] |

Ghost flights: in winter 2021-22, with the EU threshold at 50%, Lufthansa Group said it would have to run about 18,000 "unnecessary" flights to protect slots, having already cut 33,000 flights (about 10% of its winter plan); the Commission replied that 50% was "very reasonable" and that the flights carried passengers and cargo [V]. The same logic applies in normal times at 80%: a carrier weighs the cash loss of flying a thin winter rotation against the capital value of the series (section 2).

### 1.6 United States

- **High Density Rule (HDR)**, 14 CFR 93 Subpart K: promulgated 1968, in force 1969, for five airports — JFK, LaGuardia, Newark, Chicago O'Hare, Washington National [V]; Newark's limits were suspended in 1970 [R]. Slots were hourly quotas by class (air carrier / commuter / other).
- **Buy-sell rule** 1986 (above). Use-or-lose was originally 65% over two months, raised to 80% in 1992 [R].
- **AIR-21 (2000)** phased the HDR out: O'Hare after 1 July 2002; JFK and LaGuardia on 1 January 2007 [V]. DCA stayed under Subparts K and S.
- **Replacement FAA orders** after congestion returned:
  - **JFK**: 81 scheduled operations per hour, 06:00-22:59 Eastern [V]. Order extended to 28 October 2028 [V].
  - **LaGuardia**: 71 scheduled Operating Authorizations per hour plus 3 unscheduled reservations [V] (75 scheduled under the 2006 order, cut by attrition from 2009 [R]). Extended to 28 October 2028 [V]. The Port Authority's 1,500-mile perimeter rule also applies (Saturdays and Denver excepted) [R].
  - **Newark**: 81/hour order from 2008; lifted and redesignated Level 2 from 30 October 2016 [R]. After the 2025 TRACON outages and runway works the FAA imposed an interim order: 28 arrivals and 28 departures per hour during construction, 34 each otherwise until October 2025; 36 each (72 total, up from 68) from 26 October 2025 to 24 October 2026; extended at 36/36 from 25 October 2026 to 30 October 2027 [V].
  - **O'Hare**: temporary arrival caps 2004-2008 [R]; now Level 2 [R]. LAX and SFO are also Level 2 [R].
  - FAA's 2008 plan to auction New York slots (Congestion Management Rule, 73 FR 60574) was stayed by the D.C. Circuit and rescinded in 2009 [V for the rule, R for outcome].
- **DCA**: slots plus a perimeter. The perimeter was 650 statute miles from 1966, 1,000 from 1981, 1,250 from 1986 [V]. Beyond-perimeter "slot exemptions" were added by statute: 2000 (AIR-21: 24 exemptions, 12 beyond / 12 within), 2003 (Vision 100), 2012 (8 new beyond-perimeter plus conversions) — together 40 daily beyond-perimeter flights (20 round trips) [V]. The FAA Reauthorization Act of 2024 added 10 more (5 round trips), awarded in December 2024 to Alaska, American, Delta, Southwest and United, two each [V]. GAO: DCA's maximum in any hour is 67 operations and it routinely operates below that [V].
- **GAO-12-902 (September 2012)**: flights at the four slot-controlled airports were 75% more likely to be scheduled with aircraft of fewer than 100 seats than at comparable non-slot airports; the 80% rule (applied to pooled holdings) lets capacity go unused and favours large holders [V].
- United held 902 of 1,233 Newark slots (73%) in 2015 according to the DOJ complaint [V].

### 1.7 China, India, Japan

- **Japan**: Haneda slots are allocated by the transport ministry (MLIT), not traded. International daytime slots are first negotiated country-by-country in bilaterals, then split between ANA and JAL. October 2013: 16 daytime international pairs, ANA 11 / JAL 5 [V]. 2019-20 expansion: 50 daytime pairs, 25 to Japanese carriers (ANA 13.5, JAL 11.5; of which US routes 6 pairs) and 25 to foreign carriers [V]. Value: Sankei (2013) put the annual revenue of one daytime Haneda slot at about JPY 10 billion [V]; profit per slot was widely quoted at roughly a tenth of that [R, low confidence].
- **China**: CAAC's 2018 slot measures (59 articles) rank historic slots first, changes second, new entrants third [V]; about 21 "main coordinated airports" [R]. A 2015-16 pilot allocated half of 196 new weekly domestic slots at Guangzhou (auction of 9 pairs, three-year rights, no renewal guarantee) and Shanghai Pudong (lottery) by market mechanisms; IATA objected and estimated winners would barely break even by year three [V].
- **India**: slots are allocated by airport operators and the AAI under 2013 ministry guidelines modelled on the WSG; no cash trading. When Jet Airways stopped flying in 2019 its ~700 slot pairs were reallocated "temporarily" and the government's position was that slots are not an airline's property [R].

---

## 2. What a slot is worth

Unit convention: a "daily pair" is one arrival plus one departure, every day, both seasons = 2 x 7 x 2 = 28 weekly-season slot units. Frontier Economics' 2019 Heathrow study used the single-unit price (GBP 812,000) across 18,500 units to value all Heathrow slots at GBP 15 billion [V, as reported in the IAG paper]; that is about GBP 22.7 million per daily pair (derived).

### 2.1 Heathrow transactions

| Date | Buyer | Seller | Daily pairs | Price | Per pair | Time of day / note | Tag |
|---|---|---|---|---|---|---|---|
| 1998 | British Airways | Air UK | 4 | GBP 15.6m | GBP 3.9m | | V (a) |
| 2002 | British Airways | SN Brussels | 7 | GBP 27.5m | GBP 3.9m | | V (a) |
| 2003 | British Airways | SWISS | 8 | GBP 22.5m | GBP 2.8m | | V (a) |
| 2003 | British Airways | United | 2 | GBP 12m | GBP 6m | | V (a) |
| 2004 | Virgin Atlantic | Flybe | 4 | GBP 20m | GBP 5m | | V (a) |
| 2004 | Qantas | Flybe | 2 | GBP 20m | GBP 10m | | V (a) |
| 2006 | British Airways | BWIA | 1 | GBP 5m | GBP 5m | | V (a) |
| 2007 | British Airways | Malev | 2 | GBP 7m | GBP 3.5m | | V (a) |
| 2007 | British Airways | (ex-British Mediterranean slots, sold by bmi) | 7.3 | GBP 30m | GBP 4.1m | | V (a); seller identity R |
| 2007 | Virgin Atlantic | Air Jamaica | 1 | GBP 5.1m | GBP 5.1m | | V (a) |
| 2007 | undisclosed | Alitalia | 3 | GBP 67m | GBP 22.3m | | V (a) |
| 2008 | Continental | GB Airways, Alitalia, Air France | 4 | USD 209m (GBP 104.5m) | USD 52m / GBP 26.1m | Open Skies entry; pre-crisis peak | V |
| 2012 | IAG | Lufthansa (whole of bmi) | up to 56 | GBP 172.5m for the airline | about GBP 4m implied | Loss-making airline attached; 12 pairs surrendered as remedy | V |
| 2013 | Delta | undisclosed | 2 | GBP 30.8m | GBP 15.4m | | V (a) |
| Feb 2013 | Etihad | Jet Airways | 3 | USD 70m (GBP 46.2m) | USD 23m | Sale and leaseback; Jet handed them over for good in 2019 | V |
| 2014 | American | Cyprus Airways | 1 | USD 31m | USD 31m | Cyprus Airways' last LHR pair; its other pair went to Middle East Airlines | V |
| Feb 2015 | Turkish Airlines | SAS | 1 | USD 22m | USD 22m | | V |
| Feb 2015 | "major international carrier" (reported as American) | SAS | 1 | USD 60m | USD 60m | Then the record | V |
| Feb-Mar 2016 | Oman Air | Kenya Airways (routed through Air France-KLM) | 1 | USD 75m (reported GBP 50-58m) | USD 75m | Early-morning arrival; Kenya Airways kept flying on a slot leased from KLM | V |
| Jan 2017 | Delta | Croatia Airlines | 5/7 (five days a week) | USD 19.5m | about USD 27m daily-equivalent | Mid-morning; leased back for summer 2017 | V |
| Mar 2017 | undisclosed | SAS | 2 | USD 75m | USD 37.5m | SAS could keep using them for up to 3 years; SAS went from 19 to 17 pairs | V |
| 2020 | undisclosed | Air New Zealand | 1 | USD 27m | USD 27m | Sold when LAX-LHR ended; pandemic-era price | V (OAG) |
| 2023 | British Airways | Flybe (in administration) | 7 | nil | — | Remedy slots; reverted to BA, not saleable separately | V |

(a) = table reproduced on Wikipedia "Landing slot" from a UK consultancy compilation; values rounded.

Further Heathrow evidence:

- CAPA: trades averaged about GBP 4m per daily pair to the mid-2000s; a run of trades above GBP 20m in 2007-08; two 2013 deals at GBP 15m [V].
- IAG's count of publicly reported trades 2013-2025: 5, 7, 8, 2, 2 5/7, 0, 0, 1, 0, 0, 0, 1, 0 daily pairs per year — an average of 2.1 pairs a year, 0.3% of slots in use. Reported prices vary about 8x between highest and lowest. Some have changed hands for GBP 40m or more [V].
- bmi carried its 77.7 daily pairs at GBP 770m in 2007 (GBP 9.9m a pair) [V]; Aviation Strategy (June 2008) valued bmi's 543 weekly pairs at GBP 678m-1.26bn [V].
- 2025 press reports say morning pairs were again changing hands at up to USD 75m [V, secondary sources].

**Price by time of day.**

- Heathrow's commercial head (quoted by Aviation Week): an early-morning daily pair about GBP 15m, falling 30% by midday and 50% by evening [V]. CAPA/OAG: midday about 30% below peak, mid-afternoon a further 20% lower; Flybe's later slots "less than 50% of peak" [V].
- Aviation Economics (2008) estimates per **departure** slot, GBP m: 06-07h 1-3; 07-08h 2-3; 08-09h 3-4; 09-13h 5-7.5; 13-14h 3-7.5; 14-18h 5-10; 18-19h 6-12; 19-20h 5-10; 20-21h 4-7; 21-22h 1-4 [V]. Pair value is about twice these. Note the asymmetry: the most valuable **arrival** slots are 05:00-08:00 (overnight long-haul arrivals feeding the morning bank) while very early **departure** slots are the cheapest.
- Buyer type dominates. Using Oliver Wyman's 2024 Heathrow figures (long-haul EBIT GBP 62 per passenger, long-haul margin 10.9%, short-haul 2.3%), a long-haul slot pair throws off GBP 12-17m of free cash flow a year and supports a GBP 40m purchase at an 8.2% WACC; a short-haul operation cannot [V, IAG paper].

**Slots as collateral.** Virgin Atlantic: GBP 220m senior secured bonds backed by Heathrow slots (December 2015, Europe's first slot securitisation; slots would move to a subsidiary airline on default) [V]; a GBP 170m hedge-fund loan secured on other slots and aircraft inside the GBP 1.2bn 2020 recapitalisation [V]; USD 745m (about GBP 600m) from Apollo in 2025 secured on the slot portfolio [V]. IAG back-solves the 2025 deal to a whole-airport slot value of GBP 25-30bn [V]. US majors pledge slots, gates and routes to secured loans and loyalty financings [V, 10-K debt notes].

**Lease rates.** No public rate card exists. **ESTIMATE**: 6-10% of capital value per year (roughly GBP 1-3m a year for a GBP 15-30m pair), from the perpetuity logic above and from sale-and-leaseback terms.

### 2.2 Gatwick and other UK

| Date | Buyer | Seller | Slots | Price | Tag |
|---|---|---|---|---|---|
| 2013 | easyJet | Flybe | 25 Gatwick pairs | GBP 20m (GBP 0.8m a pair) | R |
| Nov 2017 | IAG | Monarch (administrators) | Monarch's Gatwick portfolio | undisclosed; Monarch's Gatwick plus Luton slots were estimated at about GBP 60m | V |
| Nov 2017 | Wizz Air | Monarch | Luton portfolio | undisclosed | V |
| Nov 2019 | easyJet | Thomas Cook (liquidator) | Gatwick 12 summer + 8 winter pairs; Bristol 6 summer + 1 winter | GBP 36m | V |
| Nov 2019 | Jet2 | Thomas Cook | Manchester, Birmingham, Stansted | undisclosed | V |
| 2021 | Wizz Air | Norwegian | 15 daily Gatwick pairs | undisclosed | R |

Derived: the Thomas Cook Gatwick package implies very roughly GBP 2-3m per year-round-equivalent Gatwick pair in 2019 (**ESTIMATE**; summer slots carry most of the value).

### 2.3 United States

| Date | Deal | Slots | Price | Per pair | Tag |
|---|---|---|---|---|---|
| Dec 2008 | Southwest buys ATA's certificate and slots in bankruptcy | 14 LGA slots (7 pairs) | USD 7.5m | about USD 1.1m | R |
| 2010 | United/Continental merger: Newark slots leased to Southwest at DOJ's instance | 36 EWR slots (18 pairs) | undisclosed | — | V |
| 2010 | JetBlue / American swap | 8 DCA pairs to JetBlue for 12 JFK pairs to American | no cash reported | — | R; the "16 JetBlue slots" appear in the 2013 decree [V] |
| 2011 | Delta / US Airways swap | Delta gets 132 LGA pairs; US Airways gets 42 DCA pairs plus Sao Paulo rights | Delta also paid USD 66.5m | — | V |
| Dec 2011 | FAA blind auction of the swap divestitures (proceeds to Delta / US Airways) | JetBlue: 8 DCA pairs | USD 40m | USD 5.0m | V |
| | | JetBlue: 8 LGA pairs | USD 32m | USD 4.0m | V |
| | | WestJet: 8 LGA pairs (JetBlue was high bidder on all three bundles but limited to one per airport) | USD 17.6m | USD 2.2m | V |
| 2013-14 | American / US Airways merger decree | 104 DCA slots (52 pairs) and 34 LGA slots (17 pairs), plus gates at ORD, LAX, BOS, MIA, DAL | American reported USD 381m cash; DOJ put the value above USD 425m | USD 5.5-6.2m blended (derived) | V |
| | Buyers | Southwest 28 DCA pairs + 11 LGA pairs; JetBlue 20 DCA pairs (8 already held by swap); Virgin America 4 DCA + 6 LGA pairs | JetBlue: USD 75m for 12 DCA pairs | USD 6.25m | buyers V; JetBlue price R |
| Jun 2015 | United to take 24 Newark slots from Delta; Delta to take JFK slots from United | 24 EWR slots | USD 14m each way | about USD 1.2m per EWR pair | V |
| | Outcome | DOJ sued (November 2015); abandoned April 2016 when the FAA de-slotted Newark; the JFK leg went ahead | | | V |

Other bidders in the 2011 auction: Allegiant, Frontier, Sun Country, Southwest, Spirit [V].

**Balance-sheet carrying values (indefinite-lived intangibles).**

| Carrier | Item | Carrying value | Tag |
|---|---|---|---|
| Delta | International routes and slots (Pacific authorities, Asian slot airports, Heathrow) | USD 2,583m at year-end 2023, 2024, 2025 | V (10-K) |
| Delta | Domestic slots | USD 622m | V |
| United | Route authorities / airport slots | about USD 1,020m / about USD 560m | R |
| American | Domestic airport slots / international slots and route authorities | about USD 365m / about USD 708m | R |
| JetBlue | Slots at high-density airports | about USD 139m | R |

These are acquisition-accounting values (Northwest, Continental, US Airways, etc.), not marks to market; slots allocated free are carried at nil.

### 2.4 How regulators use slots as remedies

| Case | Remedy |
|---|---|
| BA / American / Iberia transatlantic JV (2010) | EC commitments: Heathrow or Gatwick slots for extra competitor frequencies to New York, Boston, Dallas and Miami; US DOT required 4 Heathrow pairs to be leased to rivals [R] |
| IAG / bmi (2012) | 12 daily Heathrow pairs surrendered; all taken by Virgin Atlantic (Little Red: Edinburgh, Aberdeen), which closed in 2015; later used by Flybe (7 pairs, 86 weekly slots at the 2022 relaunch) and reverted to BA in 2023 [V] |
| US Airways / Delta (2011), American / US Airways (2013) | Divestiture of slot pairs to low-cost carriers by auction or negotiated sale (table above) [V] |
| United / Continental (2010) | 36 Newark slots to Southwest [V] |
| Air Berlin break-up (2017) | Lufthansa agreed about EUR 210m for LGW and Niki (81 of 144 aircraft); cleared for LGW only after giving up Dusseldorf slots; easyJet paid about EUR 40m for the Tegel operation, slots and leases on up to 25 A320s, cleared unconditionally on 12 December 2017; General Court upheld both in October 2021 [V] |
| Alitalia to ITA (2021) | ITA allowed to take 85% of Alitalia's Linate slots and 43% of its Fiumicino slots [R] |
| Korean Air / Asiana (2022-24) | EU: T'way given slots, traffic rights and support to fly Seoul-Barcelona, Frankfurt, Paris, Rome; UK CMA: up to 7 weekly Heathrow slot pairs to Virgin Atlantic; Korea's KFTC: slots and rights on 34 routes to rivals (Air Premia, United, T'way on the first 10) [V] |
| Lufthansa / ITA (2024-25) | Approved with remedies; remedy-takers approved 29 November 2024: easyJet short-haul (Linate slots — the first offer was 11 daily pairs — and Fiumicino; it based 5 aircraft at Linate and 3 at Fiumicino from spring 2025); IAG and Air France-KLM long-haul via slots and interline access [V] |
| IAG / Air Europa | Abandoned August 2024 after remedy talks failed [R] |

Lesson for a simulator: authorities value slot remedies by number of daily frequencies needed to replace the lost competitor on overlap city pairs, and hand them to a named entrant at nil or low price with a use obligation (typically 3-6 seasons on the remedy route before the slots become unrestricted).

---

## 3. Slots under disruption and as a commercial weapon

- **Hoarding by gauge and frequency.** The 80% rule is satisfied by any aircraft, so the cheapest way to hold a slot is a small aircraft on a short sector. GAO found slot-controlled US airports 75% more likely to see sub-100-seat aircraft [V]; LaGuardia's regional-jet shuttle patterns and Heathrow's surviving short-haul feeders are the textbook cases. Heathrow short-haul earns a 2.3% margin against 10.9% long-haul [V], which is tolerated because the slots feed, or can later be converted to, long-haul.
- **Slot sitting / babysitting.** A holder with no immediate use leases the slot to a partner or operates a placeholder service; historic rights stay with the holder (WASG 8.13) [V]. Kenya Airways sold its pair and leased one from KLM [V]; Air France leased out a Heathrow slot in late 2025 rather than use it [V, IAG paper].
- **Retreat of secondary carriers from Heathrow** 2005-2025: Iberia departures down about 51%, Air France-KLM 49%, SAS 34%, TAP 17%; Cyprus Airways, Luxair, TAROM and airBaltic left entirely — mostly by selling or redeploying slots toward long-haul users [V, IAG paper].
- **Winter thinness.** Summer and winter historics are separate rights, each earned only by flying that season. Carriers therefore operate thin winter rotations at a loss to keep next winter's series (and with it a year-round pattern). The 2021-22 ghost-flight row is the extreme case [V].
- **Blocking entry.** Incumbents retime (historic retimings rank ahead of the pool) and absorb pool slots through the non-new-entrant 50%. New entrants at full airports receive scattered off-peak times; at Heathrow buying has been the realistic entry route [V].

**Insolvency.**

| Case | What happened to the slots |
|---|---|
| Monarch (October 2017) | ACL refused to allocate summer 2018 slots to a grounded airline; the High Court agreed; the Court of Appeal reversed on 23 November 2017 (*Monarch Airlines Ltd v Airport Coordination Ltd* [2017] EWCA Civ 1892): an airline in administration that still holds its licence is entitled to its historic slots, which the administrators could then exchange. Gatwick slots went to IAG, Luton slots to Wizz; the portfolio had been estimated at about GBP 60m [V] |
| Thomas Cook (September 2019) | Liquidator sold all UK slots within seven weeks: easyJet GBP 36m (Gatwick, Bristol); Jet2 undisclosed (Manchester, Birmingham, Stansted) [V] |
| Flybe (March 2020; January 2023) | Heathrow slots were merger-remedy slots: usable but not saleable apart from the business. The 2020 owner lost them when the old operating licence was revoked [V, press]; the relaunched Flybe's 7 Heathrow pairs reverted to BA and its 5 Schiphol pairs to the Dutch coordinator in 2023, despite interest from Lufthansa and Air France-KLM in buying the company for them [V] |
| Air Berlin (2017) | Slots were the main asset in the EUR 210m and EUR 40m sales (2.4) [V] |
| Alitalia (2021) | Slots transferred administratively to ITA under a Commission-agreed cap [R] |
| ATA (2008) | Bankruptcy court sale of 14 LGA slots to Southwest [R] |

Legal character: under EU/UK law a slot is a permission, not property, yet it can be exchanged for value, charged in substance (through a subsidiary-airline structure) and realised by an office-holder if the airline keeps its licence long enough. Under US rules a slot is an operating privilege subject to FAA withdrawal, yet bankruptcy courts have long treated it as an asset of the estate that can be sold [R].

---

## 4. ATFM slots on the day

### 4.1 Europe: EUROCONTROL Network Manager and CASA

Source for the rules: ATFCM Users Manual, edition Wave 2.6 (validity 1 December 2026) [V].

**Creating a regulation.** A Flow Management Position (FMP) asks the NM Operations Centre to regulate a traffic volume (a sector, point or aerodrome). The regulation has start and end times, a location, an entry **rate** and optionally sub-periods with different rates. CASA builds an empty **Slot List**: for example a four-hour sub-period at 28 flights/hour is 112 slots about 2 minutes apart.

**Allocation ("first planned, first served").**

1. *Pre-allocation.* As flight plans arrive, each affected flight gets a provisional slot in order of its Estimated Time Over (ETO) the restricted location. If the slot nearest its ETO is free it takes it with no delay; if that slot is pre-allocated to a flight with a later ETO, the new flight takes it and the displaced flight looks for the next one (chain reaction). Provisional slots change constantly.
2. *Allocation.* At **SIT1 = EOBT minus 2 hours** the slot is allocated and a **Slot Allocation Message (SAM)** carrying the **CTOT** goes to the operator and the tower. An allocated slot cannot be taken by another flight (unless the regulation is deep-rectified).
3. *Several regulations.* The flight takes the delay of the **most penalising** regulation and is forced with that delay into the others.
4. *Late filers* (flight plan less than 3 h before EOBT) may be penalised. *Late updaters* (DLA/CHG less than 2 h before EOBT) are served from a reserved "pending rate" and can only get the next available later slot, never an earlier one.

**Compliance.** ATC has a **slot tolerance window of -5 to +10 minutes** around CTOT. A flight that cannot make it must send a DLA; a tower may ask for one **slot extension of at most 10 minutes**, only within 20 minutes of the calculated off-block time and only if no overload results. (FAA EDCT compliance is +/- 5 minutes [V, NBAA].)

**What happens to the slot when a flight is cancelled or delayed.**

- The flight's place is released to the list. There is **no ownership**: the operator of the cancelled flight has no claim on the freed capacity.
- *Slot amendment procedure*: for flights still pre-allocated (more than 2 h from EOBT), the list is re-sorted and later flights move up.
- *True Revision Process*: every minute, from SAM issue until a parameter before CTOT, CASA tries to improve already-allocated flights. A flight in status **RFI** (Ready For Improvement, the default) or **REA** (Ready, set by ATC when doors are closed, up to 15 minutes before EOBT) is improved directly by a **Slot Revision Message (SRM)**. A flight in status **SWM** instead receives a **Slot Improvement Proposal (SIP)** that the operator must accept or reject by a response time; unanswered, it lapses and the capacity is offered on.
- *Recover Hole mechanism*: when a regulated flight sends a delay, flights with an earlier ETO in RFI/REA status are moved into the hole so that the delaying flight can find a non-disproportionate later slot.
- Improvements must respect taxi time and the airport's TIS (time to insert into sequence); TRS (time to remove from sequence) stops last-minute deteriorations. Flights within about 30 minutes of off-blocks are not recalculated when a regulation changes.
- **SLC (Slot Requirement Cancellation)** tells the operator it is no longer regulated (regulation cancelled, flight rerouted out of it, or plan cancelled).

**Slot swapping.** Requested through the e-Helpdesk, a tower or an FMP. Conditions: both flights have slots issued; both are subject to the same most-penalising regulation; the operator may only swap its own flights (or flights of an operator with which it has a formal agreement); **up to three swaps per flight**. During critical events at an airport FMPs may swap flights of different operators. Requests before SIT1 are rejected automatically.

**UDPP.** SESAR's User-Driven Prioritisation Process extends this: enhanced slot swapping (in NM systems since 2017), fleet delay apportionment, selective flight protection and margins, letting an airline reorder its own flights inside its share of a regulation without harming others [R]. It moves Europe towards the US notion that an airline has an entitlement to "its" delay positions, but in today's operations the three-swap rule is the limit.

**Exemptions.** Head of State, search and rescue, medical evacuation, fire-fighting and state-authorised ATFMX flights; flights departing from outside the ATFCM area and its adjacent area (so inbound long-haul is not held on the ground and simply consumes capacity); flights already airborne when a regulation is created are forced in without delay [V].

**Delay statistics** (EUROCONTROL area, PRR 2024) [V]:

| Year | En-route ATFM delay per flight (min) |
|---|---|
| 2012 | 0.62 |
| 2013 | 0.53 |
| 2014 | 0.60 |
| 2015 | 0.73 |
| 2016 | 0.86 |
| 2017 | 0.88 |
| 2018 | 1.74 |
| 2019 | 1.57 |
| 2020 | 0.33 |
| 2021 | 0.29 |
| 2022 | 1.74 |
| 2023 | 1.80 |
| 2024 | 2.13 (worst since 2001) |

- 2024: 22.4 million minutes of en-route ATFM delay (+24% on 2023, +30% on 2019) with traffic still 4.3% below 2019; 19.8% of flights regulated, 12.6% delayed, 4.8% delayed more than 15 minutes; estimated cost EUR 2.8 billion (about EUR 125 per minute, derived; the PRR used EUR 127 per minute for 2023) [V].
- Cause shares 2024: ATC capacity 38.8%, adverse weather 36.2%, ATC staffing 13.0%, other 5%, ATC disruptions (strikes, technical) 3.6%, special events and projects 2.5%. In minutes per flight: capacity 0.83, weather 0.77, staffing 0.28, other 0.11, disruptions 0.08, projects 0.05, airspace management 0.02. ANSPs themselves say over 80% of delay coded to capacity or weather had a staffing element. Weather-coded delay rose from 0.87 million minutes in 2012 to 8.1 million in 2024 [V].
- Airport arrival ATFM delay at the top 30 airports: 1.36 minutes per arrival in 2024 (+0.14 on 2023, +0.19 on 2019). Worst: Lisbon 4.20, Amsterdam 3.76, Heathrow and Gatwick 3.60, Athens 3.46 [V].
- Summer peaks are far higher: June-August 2024 the network averaged 4.6 minutes of ATFM delay per flight [V]. All-cause departure delay averaged 17.7 minutes per flight in 2024 (18.0 in 2023) [V]; ATFM delay is a minority of total delay, the rest being reactionary and airline causes.

**Cost of delay** (EUROCONTROL Standard Inputs for Economic Analyses, release 10.0.2, August 2026, 2022 prices; underlying study University of Westminster "European airline delay cost reference values" v4.1, December 2015, first issued 2004 and updated 2011; no newer base study had been adopted) [V]:

| Phase | Tactical, all delays incl. reactionary (EUR/min) | Tactical, delays under 30 min (EUR/min) | Strategic (schedule buffer) (EUR/min) |
|---|---|---|---|
| At gate | 166 | 45 | 17.78 |
| Taxi | 182 | 62 | 46.69 |
| En-route (cruise extension) | 212 | 89 | 82.95 |
| Arrival management | 206 | 84 | — |

Network average cost of one minute of ATFM delay: **EUR 100** (the long-quoted figure; presentations have cited EUR 102-104) [V].

Tactical cost of a delay of given length, base scenario, with network effect, EUR per event [V]:

| Type | At gate 5 / 15 / 30 min | Taxi 5 / 15 / 30 | En-route 5 / 15 / 30 |
|---|---|---|---|
| AT43 | 36 / 213 / 724 | 71 / 309 / 915 | 83 / 345 / 986 |
| AT72 | 47 / 286 / 974 | 83 / 392 / 1,199 | 107 / 463 / 1,343 |
| DH8D | 47 / 297 / 1,057 | 83 / 404 / 1,272 | 130 / 534 / 1,520 |
| E190 | 71 / 380 / 1,366 | 130 / 558 / 1,722 | 213 / 832 / 2,269 |
| A319 | 83 / 523 / 1,901 | 154 / 724 / 2,305 | 286 / 1,128 / 3,112 |
| A320 | 95 / 594 / 2,162 | 178 / 844 / 2,672 | 297 / 1,199 / 3,386 |
| B738 | 107 / 641 / 2,305 | 178 / 856 / 2,745 | 321 / 1,283 / 3,599 |
| A321 | 119 / 689 / 2,566 | 201 / 927 / 3,041 | 357 / 1,414 / 4,015 |
| B752 | 119 / 736 / 2,720 | 237 / 1,093 / 3,445 | 404 / 1,592 / 4,431 |
| B763 | 201 / 1,069 / 3,802 | 345 / 1,497 / 4,656 | 606 / 2,281 / 6,225 |
| A332 | 213 / 1,176 / 4,218 | 404 / 1,722 / 5,310 | 677 / 2,542 / 6,961 |
| B744 | 286 / 1,627 / 5,939 | 546 / 2,400 / 7,484 | 1,104 / 4,086 / 10,857 |

The cost is strongly convex in delay length (passenger care, missed connections, crew and reactionary delay grow faster than linearly). The study has no 787, A350, A220 or neo values.

Cost of cancelling a flight on the day (same source, 2022 prices) [V]: 50 seats EUR 6,790; 120 seats EUR 16,640; 180 seats (network carrier) EUR 25,720; 189 seats (low-cost) EUR 18,570; 250 seats EUR 85,570; 400 seats EUR 123,900; system-wide average EUR 20,930. Passenger care and compensation is EUR 3,100 / 7,600 / 12,400 / 17,500 / 40,500 / 64,800 of those totals.

### 4.2 United States: Ground Delay Programs under CDM

**Program.** When arrival demand will exceed an airport's Airport Acceptance Rate (AAR) for a sustained period the Command Center issues a GDP: arrival slots at the AAR for each hour, over a program window, for flights within a **scope** defined by distance, by ARTCC tier or by named centers [V, NBAA]. Programs apply to departures from the contiguous US and selected Canadian airports [V]; flights already airborne, flights outside the scope and other international departures are exempt and are fitted in first [R]. Each controlled flight receives an Expected Departure Clearance Time (EDCT = slot time minus flight time); compliance window +/- 5 minutes [V].

**Ration-by-Schedule (RBS).** Slots are assigned in order of **original scheduled arrival time**, not current estimated time. That was the foundational CDM bargain of the late 1990s: an airline that honestly reports a delay or cancellation no longer loses its place [V for the three-step structure: FAA initial allocation by RBS; airline cancellations and substitutions; FAA compression — Vossen, Ball et al.].

**Ownership and substitution.** After RBS each slot "belongs" to the airline whose flight received it.

- **Cancel-and-substitute**: an airline that cancels a flight **keeps control of that flight's slot** and may put any other of its own flights into it, provided that flight can physically arrive by then; it may also reshuffle its flights among its own slots.
- **Slot Credit Substitution (SCS)**: a conditional, single-transaction request — the carrier offers to give up slot s1, by cancelling or delaying f1, on condition that it gets a slot inside the window t1 to t2 for f2. The system executes it by bridging: intervening flights of other carriers each move up one position, the requester gets the slot it wanted, and nobody is made later. Introduced because batch compression is periodic and not guaranteed [V for the concept; mechanics R].
- **Compression** (batch, run by the Command Center). For each open slot *s* owned by airline A (vacated by a cancellation or by a flight that can no longer make it):
  1. Fill *s* with A's own next flight that can use it. The slot that flight leaves becomes the open slot, still A's; repeat.
  2. If A has none, move up the next flight of another CDM carrier B that can use *s*. **A becomes the owner of the slot B's flight vacated**; return to step 1 with that slot.
  3. If nobody can use it, the slot goes unused.
  The releasing airline is thus paid with a later slot it can use; all other flights move earlier or stay put. Both compression and SCS are one-for-one trades of a slot worth nothing to its owner for one worth something [V, Vossen/Ball/Montebello; step detail R].
- **Adaptive Compression** (2007): a background process that continuously finds slots about to go unused and does the same swap automatically, so EDCTs can move earlier without a manual compression run [V, NBAA; date R].
- **Unified Delay Program (UDP)**, used for virtually all GDPs since 2011: holds back reserve slots by a historical factor for pop-up flights; a pop-up gets the average delay of its 15-minute bin and then the next open reserve slot [V].
- **Ground Stop**: all departures to the airport within scope are held at origin, typically for 30-120 minutes, usually ahead of or instead of a GDP; no slot ownership [R]. **Airspace Flow Program (AFP)** (2006): the same RBS/substitution machinery applied to a Flow Constrained Area in en-route airspace [R]. **CTOP** (2014): several FCAs at once, with airlines filing Trajectory Options Sets so the system can trade delay against reroute [R].

**Typical numbers.** No official per-airport GDP table could be retrieved in this session. **ESTIMATE** from recalled FAA/academic tabulations: about 1,000 GDPs a year nationally; Newark 150-250, San Francisco 120-200, LaGuardia 120-200, JFK 60-120, Philadelphia and Boston 40-90, O'Hare 40-90; average assigned delay per controlled flight 45-70 minutes. Verified snapshots of bad days in 2023-24: Newark programs averaging 136-143 minutes, LaGuardia 56-62, JFK 90 [V, advisory summaries].

**Cost of delay, US.**

- NEXTOR "Total Delay Impact Study" (2010): total cost of US flight delay in 2007 **USD 32.9 billion**, revised in December 2010 to **USD 31.2 billion**: airlines 8.3, passengers 16.7, lost demand 2.2 (3.9 in the first release, derived), indirect GDP effect 4.0. Schedule buffers cost a further USD 3.7bn to airlines and USD 6bn to passengers [V].
- Airlines for America, direct aircraft operating cost per block minute, US passenger airlines (Form 41) [V unless tagged]:

| Year | Total USD/min | Crew | Fuel | Maintenance | Ownership | Other |
|---|---|---|---|---|---|---|
| 2017 | 68.48 [R] | | | | | |
| 2018 | 74.20 [R] | | | | | |
| 2019 | 74.24 [R] | | | | | |
| 2023 | 100.80 [V, mirror of A4A page] | 32.68 | 37.27 | 16.88 | 9.97 | 4.00 |
| 2024 | 100.76 | 35.23 | 33.06 | | | |
| 2025 | 98.41 | 37.01 | 29.34 | 18.35 | 9.76 | 3.95 |

  FAA-recommended value of passenger time: USD 47 per hour [V].

---

## 5. Airport and navigation charges

### 5.1 A common yardstick: charges per passenger for a full turnaround

The Jacobs (formerly LeighFisher) *Review of Airport Charges* prices a landing plus take-off for eight aircraft types at 50 airports and divides by passengers carried at an 83.2% load factor. Its 2024 index, published in a Jacobs note placed on the CAA website in 2025 [V]:

| Rank | Airport | SDR per passenger, 2024 (original aircraft mix) | Approx. USD |
|---|---|---|---|
| 1 | London Heathrow | 68.22 (56.39 with a newer, quieter aircraft mix) | 91 (75) |
| 2 | Auckland | 58.05 | 77 |
| 3 | Sydney | 55.90 | 74 |
| 4 | Moscow | 53.37 | 71 |
| 5 | Toronto | 48.48 | 64 |
| 6 | Mexico City | 45.58 | 61 |
| 7 | Newark | 43.51 | 58 |
| 8 | Frankfurt | 43.47 | 58 |
| 9 | Prague | 42.88 | 57 |
| 10 | Zurich | 42.40 | 56 |
| 11 | New York JFK | 41.85 | 56 |
| 12 | Los Angeles | 40.26 | 54 |
| 13 | Budapest | 40.22 | 53 |
| 14 | Cancun | 39.12 | 52 |
| 15 | Amsterdam | 36.80 (before the 41% rise of April 2025) | 49 |
| 16 | Lisbon | 36.65 | 49 |
| 17 | Vancouver | 36.12 | 48 |
| 18 | Vienna | 34.64 | 46 |
| 19 | Brussels | 33.92 | 45 |
| 20 | Singapore | 33.68 | 45 |
| 21 | Rome Fiumicino | 32.75 | 44 |
| 22 | London Gatwick | 32.38 | 43 |
| 23 | Dusseldorf | 30.42 | 40 |
| 24 | Athens | 29.77 | 40 |
| 25 | Berlin | 29.73 | 40 |
| 26 | Stockholm | 29.34 | 39 |
| 27 | Copenhagen | 28.26 | 38 |
| 28 | Milan Malpensa | 27.66 | 37 |
| 29 | Madrid | 26.46 | 35 |
| 30 | San Francisco | 26.07 | 35 |
| 31 | Hong Kong | 25.08 | 33 |
| 32 | Warsaw | 24.17 | 32 |
| 33 | Paris CDG | 23.90 | 32 |
| 34 | Miami | 23.64 | 31 |
| 35 | Osaka Kansai | 22.62 | 30 |
| 36 | Tokyo Narita | 21.02 | 28 |
| 37 | Bangkok | 20.45 | 27 |
| 38 | Washington | 20.21 | 27 |
| 39 | Helsinki | 18.57 | 25 |
| 40 | Beijing | 18.47 | 25 |
| 41 | Mumbai | 18.39 | 24 |
| 42 | Dublin | 18.12 | 24 |
| 43 | Dubai | 17.85 | 24 |
| 44 | Johannesburg | 15.72 | 21 |
| 45 | Sao Paulo | 15.10 | 20 |
| 46 | Jakarta | 14.82 | 20 |
| 47 | Seoul Incheon | 14.21 | 19 |
| 48 | Delhi | 13.95 | 19 |
| 49 | Oslo | 11.92 | 16 |
| 50 | Kuala Lumpur | 10.76 | 14 |

Notes: Heathrow's figure was GBP 70.87 per passenger on the original mix and GBP 58.62 on a mix reflecting its actual fleet [V]. The sample averages 0.80 tonnes of MTOW per passenger (1,142 t over 1,428 passengers) [V]. The index counts charges on both the landing and the take-off, including passenger and security charges collected through the ticket; government ticket taxes (APD and the like) are generally outside it [R].

Per-turn totals implied by the index (index x passengers x 1.33; **ESTIMATE**, because the index is an eight-type average and wide-bodies usually pay somewhat more per passenger than narrow-bodies):

| Airport | A320 / 737-800 (146 pax) | 777-300ER (301 pax); 787-9 about 80% of this |
|---|---|---|
| Heathrow | USD 11,000-13,000 | USD 23,000-27,000 |
| Sydney, Auckland | 10,900-11,300 | 22,400-23,200 |
| Toronto | 9,400 | 19,400 |
| Newark, Frankfurt, JFK | 8,100-8,450 | 16,800-17,400 |
| Los Angeles | 7,800 | 16,100 |
| Amsterdam (2024 level; add about 40% from 2025) | 7,150 | 14,700 |
| Singapore, Gatwick | 6,300-6,550 | 13,000-13,500 |
| Madrid, San Francisco, Hong Kong, Paris CDG | 4,650-5,150 | 9,600-10,600 |
| Narita, Bangkok | 4,000-4,100 | 8,200-8,400 |
| Beijing, Dubai, Dublin, Mumbai | 3,470-3,590 | 7,150-7,400 |
| Sao Paulo, Seoul, Delhi | 2,700-2,950 | 5,600-6,050 |
| Kuala Lumpur | 2,100 | 4,300 |

### 5.2 United States

US airports charge airlines a landing fee per 1,000 lb plus terminal rents and common-use fees, set under residual or compensatory use-and-lease agreements; there is no federal en-route or terminal navigation charge (the system is funded by ticket taxes).

**Landing fee rates.**

| Airport | Rate per 1,000 lb | Basis and date | Tag |
|---|---|---|---|
| JFK | USD 8.16 plus AirTrain charge 1.77 | max take-off weight; public / non-signatory rate from 1 Sep 2025 (7.43 before) | V, Port Authority schedule |
| Newark | USD 10.62 plus AirTrain 2.06 | max take-off weight; from 1 Sep 2025 (9.31 before) | V |
| LaGuardia | USD 19.75 | max landed weight; from 1 Sep 2025 (18.04 before) | V |
| JFK / EWR / LGA | 9.17 / 11.60 / 21.02 | July 2026, third-party compilation | V, DWU |
| Los Angeles | 6.28 | MGLW, July 2026 | V, DWU |
| Chicago O'Hare | 12.05 | July 2026 | V, DWU |
| Atlanta | 1.6275 signatory, 1.7241 non-signatory (FY2026); 1.84 (July 2026) | | V |
| Dallas/Fort Worth | 4.40 | October 2026 | V, DWU |
| Denver | 4.95 | January 2026 | V, DWU |
| San Francisco | 6.99 | July 2026 | V, DWU |
| Seattle | 5.21 | January 2026 | V, DWU |
| Boston | 6.70 | July 2026 | V, DWU |
| Miami | 1.66 | October 2026 | V, DWU |
| Houston IAH / Phoenix | 2.46 / 2.50 | July 2026 | V, DWU |
| Chicago Midway | 9.31-11.64 | 2025 rate book | V |

Other Port Authority items: Newark Terminal B federal inspection charge USD 13.90 per international passenger and general terminal charge USD 6.00 per passenger [V].

**Landing fee per arrival** (derived from the rates above; weights [R]: A320 MLW 142,200 lb / MTOW 172,000 lb; 737-800 146,300 / 174,200; 787-9 425,000 / 560,000; 777-300ER 554,000 / 775,000):

| Airport | A320 | 737-800 | 787-9 | 777-300ER |
|---|---|---|---|---|
| JFK (per take-off, incl. AirTrain) | 1,708 | 1,730 | 5,561 | 7,696 |
| Newark (per take-off, incl. AirTrain) | 2,181 | 2,209 | 7,101 | 9,827 |
| LaGuardia | 2,808 | 2,889 | — | — |
| O'Hare | 1,714 | 1,763 | 5,121 | 6,676 |
| San Francisco | 994 | 1,023 | 2,971 | 3,872 |
| Los Angeles | 893 | 919 | 2,669 | 3,479 |
| Denver | 704 | 724 | 2,104 | 2,742 |
| Dallas/Fort Worth | 626 | 644 | 1,870 | 2,438 |
| Atlanta | 262 | 269 | 782 | 1,019 |
| Miami | 236 | 243 | 706 | 920 |

**Airline cost per enplaned passenger (CPE)**, FAA CATS Form 5100-127 as compiled by DWU Consulting [V]:

| Airport | FY2024 | FY2025 | | Airport | FY2024 | FY2025 |
|---|---|---|---|---|---|---|
| Los Angeles | 30.16 | 33.85 | | Miami | 16.83 | 17.62 |
| Chicago O'Hare | 29.56 | 29.84 | | Philadelphia | 15.03 | 16.33 |
| San Francisco | 32.14 | 28.11 | | Denver | 12.76 | 15.80 |
| Boston | 21.72 | 23.50 | | Dallas/Fort Worth | 13.44 | 13.86 |
| Honolulu | 20.92 | 21.57 | | Minneapolis | 11.06 | 12.71 |
| Salt Lake City | 10.44 | 19.25 | | Orlando | 8.18 | 12.56 |
| Seattle | 18.24 | 19.06 | | Houston IAH | 10.66 | 11.17 |
| San Diego | 17.35 | 18.52 | | Washington Dulles | 12.88 | 9.56 |
| Phoenix | 7.81 | 8.62 | | Washington National | 7.73 | 7.49 |
| Las Vegas | 5.36 | 7.13 | | Charlotte | 4.74 | 6.46 |
| Atlanta | 3.93 | 4.48 | | Large-hub median | 11.06 | 12.78 |

JFK, Newark and LaGuardia do not appear in that extract; **ESTIMATE** USD 35-50 given their landing fees and the new terminal programmes (the Jacobs index puts JFK and Newark at USD 56-58 per departing passenger including international facility charges). CATS and bond-document CPEs differ in scope (Atlanta: 3.93 in CATS, 7.16 "all-in" in its official statement) [V].

Per-turn airline payment = CPE x enplaned passengers: an A320 with 128 passengers costs about USD 570 at Atlanta, 1,770 at DFW, 3,600 at San Francisco, 4,330 at Los Angeles (derived).

### 5.3 Europe

**Frankfurt, tariff valid 2026** [V, Fraport charges manual]:

| Item | Rate |
|---|---|
| Landing and take-off, weight element | EUR 2.50 per 1,000 kg MTOM per landing and per take-off (minimum charges apply below 66 t) |
| Passenger element inside the take-off charge | EUR 1.79 per passenger on board at take-off |
| Noise charge per movement, by measured category (1-16) | Cat 1 88.64; 2 105.12; 3 121.58; 4 138.06; 5 235.28; 6 411.16; 7 587.06; 8 762.94; 9 938.83; 10 1,114.71; ... 16 29,976.12 |
| Example categories (landing / take-off) | A320neo 2 / 2; 737-800 3 / 6; A320ceo 4 / n.a. in extract; 787-9 5 / n.a.; 777-300ER 7 / 9; A330-200 8 / 10; A380 9 (landing) |
| Night surcharges per movement | 22:00-22:59 and 05:00-05:59: about 65% extra (Cat 3: 79.03); 23:00-04:59: three times the day charge extra (Cat 3: 364.74) |
| Emissions | EUR 4.05 per kg NOx-equivalent in the LTO cycle |
| Passenger charge per departing passenger | 24.56 (Germany, EU, Iceland, Liechtenstein, Norway, Switzerland); 30.88 (other Europe); 34.01 (intercontinental); 16.52 (transfer / transit) |
| Security charge | EUR 1.63 per passenger on board at take-off |
| PRM levy | EUR 1.82 per departing passenger |
| Central ground-handling infrastructure (baggage system etc.) per turnaround | Regional jets 214-244; A320 / 737 family 434 (apron) - 473 (pier); 757 / 767 1,414-1,471; A330 / A350 / 777 / 787 2,348-2,665; 747 3,298-3,756; A380 4,820-5,826 |
| ... plus per departing passenger | EUR 6.27 base, scaled by passengers per tonne of MTOM (18.50 for very lightly loaded aircraft down to 4.16 for dense ones) |
| Parking | Free 22:00-05:59; e.g. stand group 3 EUR 43.33 for up to 3 h, group 9 EUR 81.24 for up to 5 h, then EUR 49-101 per extra hour |
| Growth incentive | Credit of EUR 15 per additional originating intercontinental passenger and EUR 7.50 per additional intercontinental transfer passenger above the 2024 base |

Worked turnarounds at Frankfurt (my arithmetic on the tariff; emissions and parking lines are **ESTIMATE**):

| Line | 737-800, 79 t, 160 departing EU passengers | 777-300ER, 351.5 t, 300 departing (half intercontinental O&D, half transfer) |
|---|---|---|
| Weight (2 movements) | 395 | 1,760 |
| Passenger element of take-off | 286 | 537 |
| Noise (landing + take-off) | 533 | 1,526 |
| Emissions (estimate) | 45 | 250 |
| Passenger charges | 3,930 | 7,580 |
| Security | 261 | 489 |
| PRM | 291 | 546 |
| Ground-handling infrastructure | 473 + 696 = 1,169 | 2,665 + 2,103 = 4,768 |
| Parking (estimate) | 40 | 150 |
| **Total** | **EUR 6,950 (43 per passenger)** | **EUR 17,600 (59 per passenger)**; 20,200 if all 300 are intercontinental O&D |
| of which independent of passenger numbers | 1,486 (21%), i.e. EUR 19 per tonne | 6,351 (36%), i.e. EUR 18 per tonne |

Not included: DFS terminal navigation charge, the separate passenger-screening fee (statutory cap raised from EUR 10 to EUR 15 per passenger in 2025 [R]), commercial ground handling, fuel throughput.

**Heathrow.**

- Structure [V for the principle, Jacobs]: landing charges are fixed amounts per landing by noise category (seven bands from "ultra high" to "ultra low"), not by weight; plus a NOx emissions charge; a departing-passenger charge that differs by destination (domestic / European / other) with a transfer discount; aircraft parking; and a minimum departure charge. Night movements pay a multiple [R].
- Price cap (maximum revenue yield per passenger, arrivals and departures both counted) under the CAA's H7 control: GBP 30.19 in 2022 (interim) [R]; **31.57 in 2023; 25.43 in 2024** [V]; **23.73 in 2025 and 23.71 in 2026** after the July 2024 decision implementing the CMA appeal findings (down from 25.24 and 25.28) [V, CAA, nominal estimates; the cap actually charged moves with outturn inflation and correction factors, and press reports in 2026 quote about GBP 26]. The H7 average is GBP 28.40 in 2024 CPIH prices [V].
- H8 (2027-2031) initial proposals, 31 March 2026: GBP 27.20-30.50, midpoint 28.80 (2024 CPIH prices); Heathrow asked for about GBP 33, airlines about GBP 25; real vanilla WACC 4.87%; capex envelope GBP 5.4-6.1bn against Heathrow's 9.4bn; final decision due April 2027 [V]. In July 2026 the CAA allowed recovery of GBP 320m of early third-runway costs: about 15p per passenger from 2028 rising to about 30p [V].
- Because a "passenger" in the cap is counted on arrival and on departure, GBP 24-26 of yield is about GBP 50 per departing passenger, consistent with the Jacobs all-in figure of GBP 59-71 once uncapped items (baggage, check-in, PRM, NATS terminal charge) are added.
- **ESTIMATE** of the 2025-26 tariff for modelling: landing GBP 1,700 (new-generation narrow-body), 2,500-4,000 (older narrow-body or new-generation wide-body), 9,000+ (noisiest types); departing passenger GBP 30-35 short-haul and 55-65 long-haul, transfer passengers 25% less. That gives about GBP 6,500 for an A320neo with 150 passengers and about GBP 22,000 for a 777-300ER with 300, which reproduces the Jacobs average.

**Amsterdam Schiphol.** Charges rose 41% on 1 April 2025, with +5% in 2026 and -7.5% in 2027 planned (cumulative +37% on 2024), to fund EUR 6bn of investment and recover pandemic losses; noisier aircraft pay more and night movements three to six times the day rate [V]. Applied to the 2024 index, Schiphol moves to roughly SDR 50 per passenger, second only to Heathrow in Europe (**ESTIMATE**).

**Paris CDG.** Index SDR 23.90 [V]; the low figure reflects France's practice of funding security through a separate statutory airport tax and of taxing tickets (5.7) rather than through airport charges [R]. ADP is regulated on an adjusted till that excludes retail and real estate [R].

### 5.4 Asia-Pacific, Middle East, Latin America

- **Singapore Changi** [V, CAAS announcement of 7 November 2024]: a departing origin-destination passenger pays SGD 65.20 (passenger service and security fee 46.40, aviation levy 8.00, airport development levy 10.80), rising to SGD 79.20 by April 2030 (PSSF +3 a year for four years from April 2027; aviation levy +2 in April 2027). Transit passengers: SGD 9 rising to 21 by 2030, starting 1 April 2025. Landing, parking and aerobridge (LPA) for an A320: about SGD 1,200 per landing in 2024 rising annually to about SGD 1,725 by April 2030 (a 50% rebate on the increase applied April-September 2025). **ESTIMATE** for a 777-300ER / A350 LPA: SGD 4,500-6,500.
- **Sydney / Auckland**: the most expensive in the index after Heathrow (SDR 56-58) [V]; Australia adds a government Passenger Movement Charge of AUD 70 [V].
- **Tokyo**: Narita SDR 21, Kansai 22.6 [V]; Japan's departure tax is JPY 1,000 [V]. Haneda's international passenger facility charge is around JPY 3,000 [R].
- **Beijing** SDR 18.5 [V]. China levies a civil aviation development fund of CNY 50 (domestic) / CNY 90 (international) per departing passenger on the ticket [R].
- **India**: Delhi SDR 14, Mumbai 18.4 [V]. Charges are dominated by the User Development Fee set by AERA per control period; Delhi's 2025 tariff differentiates by cabin and direction (international departing economy about INR 650, business about INR 810) [R, moderate confidence].
- **Dubai** SDR 17.9; **Sao Paulo GRU** 15.1; **Mexico City** 45.6 (high passenger TUA); **Johannesburg** 15.7 [V].

### 5.5 En-route and overflight charges

**EUROCONTROL (CRCO) formula** [R]: for each charging zone *i*,

    charge_i = unit_rate_i x distance_factor_i x weight_factor
    distance_factor_i = (great-circle km flown in zone i, less 20 km for each take-off or landing in the zone) / 100
    weight_factor = sqrt(MTOW in tonnes / 50)

Weight factors: A320 (73.5 t) 1.21; 737-800 (79 t) 1.26; 787-9 (254 t) 2.25; 777-300ER (351.5 t) 2.65.

**Unit rates, monthly adjusted, November 2026** [V, EUROCONTROL file ur-2026-11; values are published in euro-cents and converted here]:

| Zone | EUR | Zone | EUR | Zone | EUR |
|---|---|---|---|---|---|
| Moldova | 196.12 | Czechia | 78.94 | Hungary | 41.46 |
| Switzerland | 169.83 | Slovakia | 76.22 | Cyprus | 40.89 |
| Netherlands | 136.38 | Italy | 73.71 | Turkiye | 39.78 |
| Belgium-Luxembourg | 116.65 | Spain (continental) | 71.30 | Croatia | 39.19 |
| Germany | 97.89 | Austria | 66.02 | Serbia-Montenegro-KFOR | 38.82 |
| Poland | 95.63 | Slovenia | 65.42 | Ukraine | 37.33 |
| Denmark | 91.62 | Norway | 63.69 | Armenia | 35.56 |
| United Kingdom | 91.06 | Lithuania | 60.74 | Ireland | 34.70 |
| Finland | 90.40 | Latvia | 55.89 | Bulgaria | 31.97 |
| Estonia | 88.97 | Spain (Canarias) | 53.40 | Bosnia-Herzegovina | 25.40 |
| Sweden | 88.09 | Romania | 47.91 | Georgia | 22.49 |
| France | 79.58 | Albania | 45.52 | Greece | 22.39 |
| | | North Macedonia | 43.95 | Malta | 18.60 |
| | | Portugal (Lisboa) | 41.85 | Portugal (Santa Maria) | 8.16 |

The Swiss figure is far above the roughly EUR 120 of 2024 [R]; it is reported as published. Cost per 100 km (derived):

| Unit rate | A320 | 737-800 | 787-9 | 777-300ER |
|---|---|---|---|---|
| EUR 35 (Ireland) | 42 | 44 | 79 | 93 |
| EUR 80 (France) | 97 | 101 | 180 | 212 |
| EUR 98 (Germany) | 119 | 123 | 221 | 260 |
| EUR 170 (Switzerland) | 206 | 213 | 383 | 450 |

Example: a 1,500 km intra-European A320 sector at an average EUR 80 costs (1,500 - 40) / 100 x 97 = about EUR 1,420, or roughly EUR 9-10 per seat. A 777-300ER crossing 3,000 km of European airspace at an average EUR 70 pays about EUR 5,570.

Terminal navigation in Europe is charged separately per departure as unit rate x (MTOW/50)^0.7 [R]; **ESTIMATE** EUR 150-300 for an A320 and EUR 500-900 for a 777-300ER at major airports.

**FAA overflight fees** (14 CFR 187.53) [V]: flights that cross US-controlled airspace without landing or taking off in the US pay USD 61.75 per 100 nautical miles en route and USD 26.51 per 100 nm oceanic (unchanged since 1 January 2019; 58.45 / 23.15 in 2017, 60.07 / 24.77 in 2018). Not weight-related. Per km: USD 0.333 and USD 0.143.

**Russia, trans-Siberian royalties.** A Soviet-era arrangement obliged about 20 European airlines to pay Aeroflot for the right to overfly Siberia, on top of normal navigation charges. Reported scale: up to USD 300m a year from European carriers; the Commission estimated USD 420m in 2008; Aeroflot kept about USD 170m in 2012 after the state took about 15% [V]. An EU-Russia agreement tied to WTO accession was to abolish the payments from 2014 [V] but was never implemented [R]; since February 2022 the airspace has been closed to EU, UK and US carriers [R].

**Elsewhere.** ASECNA, China, India, Canada and most others use distance x weight formulas. No rate tables were verified in this session; **ESTIMATE** USD 0.3-0.6 per km for a narrow-body and USD 0.6-1.2 per km for a heavy wide-body.

### 5.6 Handling, de-icing, catering, disruption costs

- **Ground handling.** One industry-KPI source gives USD 600-1,400 per narrow-body departure for self-handling at a mid-size domestic station, wide-bodies at 2-3 times that; per passenger USD 4-8 narrow-body, USD 8-20 wide-body [V, low-grade source]. **ESTIMATE** for contracted full handling: narrow-body USD 1,000-3,000 per turn (low-cost minimal turn at a secondary airport at the bottom, legacy turn at an expensive hub at the top); wide-body USD 4,000-10,000. Frankfurt's compulsory infrastructure fee alone is EUR 434-473 plus about EUR 6 per passenger for a narrow-body and EUR 2,348-2,665 plus the same for a wide-body [V].
- **De-icing.** **ESTIMATE** USD 1,500-5,000 per narrow-body event and USD 6,000-20,000 per wide-body event, depending on fluid volume and whether anti-icing is applied.
- **Catering.** **ESTIMATE** per passenger: short-haul full-service economy USD 3-8; long-haul economy 12-25; premium economy 25-40; business 50-100; first 100-250; low-cost zero net (buy-on-board).
- **Passenger compensation and care.** EU/UK Regulation 261/2004: EUR 250 (up to 1,500 km), EUR 400 (1,500-3,500 km, and intra-EU above 1,500 km), EUR 600 (over 3,500 km) for arrival delays of 3 hours or more, cancellations notified less than 14 days ahead and denied boarding, unless caused by extraordinary circumstances (weather and ATC restrictions usually qualify, airline technical faults and own-staff strikes usually do not); meals, hotels and rerouting are owed regardless [R]. US: no delay compensation; automatic refunds for cancellations and significant changes since 2024; denied-boarding compensation of 200% / 400% of the one-way fare up to regulated caps [R]. EUROCONTROL's standard inputs put care and compensation at EUR 12,400-17,500 for a cancelled 180-189 seat flight and EUR 40,500-64,800 for a 250-400 seat flight [V].

### 5.7 Government taxes carried on the ticket

**United States** (A4A table, rates for 2026) [V]:

| Tax | 2026 rate |
|---|---|
| Domestic ticket (excise) tax | 7.5% of fare |
| Domestic flight segment tax | USD 5.30 per segment (5.20 in 2025, 5.00 in 2024 [R]) |
| International departure and arrival taxes | USD 23.40 each (22.90 in 2025, 22.20 in 2024 [R]); USD 11.70 mainland-Alaska/Hawaii |
| Passenger Facility Charge | up to USD 4.50 per enplanement, at most two per one-way trip (USD 18 round trip) |
| September 11th security fee | USD 5.60 per one-way trip |
| Customs / immigration / APHIS (international arrivals) | USD 7.39 / 7.00 / 3.84 |
| Frequent-flyer mile sales | 7.5% |
| Cargo waybill | 6.25% |
| Commercial jet fuel | 4.3 cents per gallon plus 0.1 cent LUST |

A4A's sample USD 400 domestic connecting round trip carries USD 74.79 of taxes and fees: 18.7% of the ticket, 23% of the base fare [V].

**United Kingdom Air Passenger Duty** (reduced = lowest class; standard = any other class), GBP:

| From | Domestic | Band A (to 2,000 miles) | Band B (2,001-5,500) | Band C (over 5,500) | Tag |
|---|---|---|---|---|---|
| 1 April 2024 | 7 / 14 | 13 / 26 | 88 / 194 | 92 / 202 | R |
| 1 April 2025 | 7 / 14 | 13 / 28 | 90 / 216 | 94 / 224 | R |
| 1 April 2026 | 8 / 16 | 15 / 32 | 102 / 244 | 106 / 253 | V |
| 1 April 2027 | 8.26 / 16.52 | 15.49 / 33.04 | 105.33 / 251.95 | 109.46 / 261.25 | V |

The higher rate (large private jets) is GBP 142 / 142 / 1,097 / 1,141 from April 2026 [V]. Distance is London to the destination country's capital. Direct long-haul flights from Northern Ireland are zero-rated [V].

**Germany, Luftverkehrsteuer** per departing passenger, EUR (short / medium / long distance class) [V]:

| From | Class 1 | Class 2 | Class 3 |
|---|---|---|---|
| 2011 | 8.00 | 25.00 | 45.00 |
| 2019 | 7.38 | 23.05 | 41.49 |
| 1 April 2020 | 12.90 | 32.67 | 58.82 |
| 2023 | 12.73 | 32.25 | 58.06 |
| 1 January 2024 | 12.48 | 31.61 | 56.91 |
| 1 May 2024 | 15.53 | 39.34 | 70.83 |
| 1 July 2026 | 13.03 | 33.01 | 59.43 |

**France, solidarity tax on tickets (TSBA, the "Chirac tax")** per departing passenger, EUR [V]:

| | Europe (France, EU, EEA, Switzerland) | Intermediate | Long-haul (over 5,500 km) |
|---|---|---|---|
| Economy, 2020 to February 2025 | 2.63 | 20.27 (single non-European rate) | 20.27 |
| Economy, from 1 March 2025 | 7.40 | 15 | 40 |
| Business / first, to February 2025 | 7.51 | 63.07 | 63.07 |
| Business / first, from 1 March 2025 | 30 | 80 | 120 |
| Business jets (turboprop / jet) | 210 / 420 | 675 / 1,015 | 1,025 / 2,100 |

The 2025 reform was budgeted to raise over EUR 800m more a year [V].

**Netherlands**: EUR 7.845 (2021), 7.947 (2022), 26.43 (2023) [V], 29.05 (2024) [R], 29.40 (2025) [V] per departing origin-destination passenger; transfer passengers exempt; a distance-based scheme is planned for 2027 [V].

**Others**: Austria EUR 12 flat; Hong Kong HKD 120; Japan JPY 1,000; Australia AUD 70 [V]. Sweden abolished its tax in July 2025 [R].

**Share of a low fare (derived).** On a GBP 40 one-way UK short-haul fare, GBP 15 of APD is 38%; a EUR 15.53 German tax on a EUR 50 fare is 31%; add EUR 25-45 of airport passenger and security charges and the non-airline share of a low-fare European ticket is commonly 40-60%. In the US the figure is about 19% on A4A's sample and above 25% on very low fares because several charges are fixed-dollar.

---

## 6. Airports competing for airlines

### 6.1 Ryanair-Charleroi: the reference case

Terms of the 2001 agreements, from Commission Decision 2004/393/EC (12 February 2004) [V]:

- Walloon Region: landing charge collected per boarding passenger rather than by tonnage, at a reduction of "some 50%" on the published tariff (Ryanair calculated 36.7% on average).
- Airport company (BSCA): ground handling at **EUR 1 per passenger**, about 10% of the price published for other users.
- A joint marketing company (Promocy) funded by BSCA and Ryanair, BSCA's contribution being **EUR 4 per boarding passenger**.
- One-off payments: EUR 250,000 for hotel and subsistence costs; **EUR 160,000 per new route**, up to three routes per based aircraft and a maximum of EUR 1,920,000; EUR 768,000 for pilot and crew recruitment and training; EUR 4,000 for office equipment; free offices and minimal hangar charges.
- Term 15 years. Ryanair to base two to four aircraft and operate at least three rotations per aircraft; repayment if it left.

The Commission held the landing-charge and handling discounts to be incompatible state aid but accepted start-up aid in principle if limited in intensity and duration; Ryanair was to repay about EUR 4m, which it placed in escrow [V]. The Court of First Instance annulled the decision on 17 December 2008 because the Commission had not assessed the Region and the airport together as a market-economy investor [V]. A later decision (2014) found the 2005-2010 arrangements were not aid [V]. EU guidelines since then allow start-up aid of up to 50% of airport charges on a new route for up to three years at smaller airports [R].

### 6.2 Typical incentive terms (for defaults)

- **New-route rebates**: 50-100% of landing fees in year one, tapering over two to three years (patterns such as 100 / 75 / 50 or 80 / 60 / 40); at hubs, per-passenger growth credits instead (Frankfurt: EUR 15 per additional intercontinental originating passenger, EUR 7.50 per additional intercontinental transfer passenger, plus load-factor rebates of EUR 9.50-15 per passenger above set load factors) [V for Frankfurt; pattern R].
- **Marketing support**: one-off or per-passenger contributions; Charleroi's EUR 4 per passenger and EUR 160,000 per route are the documented benchmark [V].
- **Net cost at secondary airports**: after rebates and marketing payments a large low-cost carrier typically nets EUR 1-5 per departing passenger for airport and handling combined (**ESTIMATE**), against EUR 35-45 at Frankfurt-class hubs. Ryanair's group-wide airport and handling cost is around EUR 8 per passenger [R].
- **United States**: airports may waive fees and fund marketing for new service for a limited period but may not pay airlines from airport revenue; communities therefore use **minimum revenue guarantees** funded by local business or federal Small Community Air Service Development grants (**ESTIMATE** USD 0.5-2m per route-year).
- **Essential Air Service**: in autumn 2024 DOT subsidised 177 communities (65 in Alaska, 112 elsewhere); the programme costs over USD 500m a year; communities within 175 miles of a large or medium hub must stay under USD 650 subsidy per passenger, and a USD 1,000 per passenger cap applies regardless of distance (Hawaii 850 from October 2026) [V]. Individual contracts run about USD 1-9m a year (Prescott 5.9m, Pierre 5.5m, Presque Isle 8.7m) [V]; the last published system average was about USD 74 per passenger outside Alaska (2006) [V]. The administration's FY2027 budget proposed a USD 372m cut [V].

### 6.3 Ownership and the regulatory till

- **Single till** (all airport profits, including retail and parking, offset aeronautical charges): Heathrow under the CAA (RAB-based five-year caps, privatised 1987) [V]; Dublin [R]. Lower charges for a given cost base; heavy investment still drives Heathrow to the top of the index.
- **Dual or adjusted till** (aeronautical charges recover aeronautical costs only): Frankfurt, Amsterdam, Paris, Australian and New Zealand airports under light-handed monitoring [R]. Sydney and Auckland show where this can lead (5.1).
- **US residual vs compensatory agreements**: under a residual agreement signatory airlines guarantee the airport's net cost (low and volatile fees, airline veto rights over capital spending); under a compensatory agreement the airport keeps non-airline revenue and bears traffic risk; most large hubs are hybrids [R]. Atlanta's CPE of USD 4.48 against Los Angeles's 33.85 is mostly this plus the age of the capital programme.

---

## Model recommendations

### (a) Cancelled flights: the ATFM slot and the seasonal airport slot

**Europe-style flow regulation (no ownership; released and re-rationed).**

    regulation R = (location, start, end, rate per hour [by sub-period])
    slots(R)     = one slot every 60/rate minutes
    demand       = non-exempt flights with ETO in [start, end], ordered by ETO   # first planned, first served
    for f in demand: give f the earliest free slot at or after ETO(f)
    delay(f)     = slot time - ETO(f);  CTOT(f) = ETOT(f) + delay(f)
    flight in several regulations: use the largest delay and force it into the others
    at EOBT - 2 h: freeze (SAM). Afterwards a slot can only improve, unless the regulation itself changes
    on CANCEL(f): free f's slot; every later flight that can physically make an earlier slot moves up
                  (re-sort if still more than 2 h from EOBT; otherwise improve one at a time, each minute,
                   subject to now + taxi + sequencing time). The cancelling airline gets no credit.
    on DELAY(f):  f gives up its slot and takes the next free slot at or after its new ETO (no priority);
                  flights behind the hole move up
    swap: an airline may exchange two of its own flights in the same regulation, at most 3 times per flight
    comply: take off within CTOT -5 / +10 min, else request a new slot
    exempt: flights from outside the region (long-haul inbound), airborne flights, state / medical / rescue

Suggested stochastic inputs: annual mean en-route ATFM delay 1.6-2.1 minutes per flight with 12-13% of flights delayed and about 5% delayed over 15 minutes; summer mean 4-5 minutes; airport arrival ATFM delay 1.2-1.4 minutes per arrival on average and 3.5-4.2 at the worst hubs; causes roughly 40% capacity, 35% weather, 13% staffing, 12% other.

**US-style ground delay program (ownership; owner substitutes, then compression).**

    slots       = arrival slots at 60/AAR minutes over the program window
    exempt      = airborne flights, origins outside the scope, most international; placed first at their ETA
    others      = ordered by ORIGINAL SCHEDULED arrival time (ration by schedule); earliest remaining slot each
    EDCT(f)     = slot time - flight time;  depart within +/- 5 min
    owner(slot) = airline of the flight assigned by RBS
    airline options: cancel a flight and KEEP its slot; put any of its own flights that can make it into
                     any slot it owns; reorder its own flights freely
    compression (run periodically / continuously): for each open slot s owned by A
        1. move up A's earliest later flight that can use s; the slot it leaves is the new open slot (A's)
        2. else move up the next flight of another airline B that can use s; A becomes owner of the slot
           B vacated; go to 1 with that slot
        3. else s is wasted
    pop-ups (late filers) get the average delay plus the wait for the next reserve slot

Behavioural consequence to model: in the US an airline with many flights in a program can cancel a low-value flight and move a high-value one into its slot, so large hub carriers suffer less value-weighted delay than their flight-weighted delay; in Europe a cancellation mainly helps whoever happens to be next in line, and airline control is limited to three same-regulation swaps per flight. Suggested GDP inputs (**ESTIMATE**): 150-250 program-days a year at Newark, 120-200 at San Francisco and LaGuardia, 60-120 at JFK, 40-90 at O'Hare, Boston and Philadelphia; mean assigned delay 45-70 minutes, 130+ minutes on the worst days.

**Seasonal airport slot (80/20 accounting).**

- Track each series = (airport, arrival or departure, time, day of week, season). A daily year-round rotation at one airport is 28 series-units (2 x 7 x 2).
- Operated count increments when any flight of the holder (or a lessee) uses the slot time on that date, whatever the route or aircraft.
- A cancellation after the baseline date (31 January / 31 August) increments the non-use count unless its cause is weather, airport or airspace closure, or third-party industrial action, in which case count it as operated. Airline-side causes (technical, crew, commercial) count against the series.
- At season end: usage >= 80% (25 of 31 summer weeks, 17 of 21 winter weeks) keeps the series for the next equivalent season; otherwise it goes to the pool.
- A cancelled day's slot is never banked. Optionally let the coordinator hand it to another airline as an ad hoc slot with no historic value.
- Pool allocation each season: 50% to new entrants (fewer than 7 slots that day at that airport), 50% to others, with requests for year-round extensions first; no transfer of pool slots for two equivalent seasons.
- Waiver events (pandemic, war, system-wide staffing shortage): lower the threshold for a season (historic values 0%, 50%, 64%, 70%) or let holders return up to 10% of slots with credit.
- The AI's decision to fly a loss-making rotation should compare the loss with the series value from (b).

### (b) Slot-value model

    value per daily year-round pair = B(airport) x H(hour) x G(buyer) x K(cycle)

**B, base value in USD millions (2025 prices):**

| Tier | Meaning | B | Calibration |
|---|---|---|---|
| S | Full all day, entry only by purchase or state allocation | 30 (Heathrow), 25 (Haneda daytime international) | Heathrow: Frontier average about GBP 22.7m per pair; 2013 trades GBP 15m; records USD 60-75m; bulk sale (bmi) about GBP 4m. Haneda: JPY 10bn annual revenue per daytime slot; no market price — ESTIMATE |
| A | Hard-capped or full most of the day | 3-8: DCA 7, LGA 4, JFK 3, Gatwick 3; others 4 | DCA USD 5.0-6.25m per pair (2011-14); LGA USD 2.2-4.4m; Gatwick about GBP 2-3m (2019); others ESTIMATE |
| B | Full at peaks | 0.8-2 (default 1.2) | Newark USD 1.2m per pair (2015) |
| C | Coordinated, slots usually obtainable off-peak | 0.1-0.5 (default 0.25) | ESTIMATE |
| D | Level 2 / Level 1 | 0 | |

**H, hour of the pair's arrival slot (local time):**

| Hour | H | Evidence |
|---|---|---|
| 05:00-07:59 at a long-haul hub | 1.5 (up to 2.5 for the single best pairs) | Record trades were early-morning arrivals |
| 08:00-10:59 | 1.0 | Reference "morning" value |
| 11:00-15:59 | 0.7 | "30% lower by midday" |
| 16:00-19:59 | 0.7-0.8 | Aviation Economics shows a second peak for evening departures |
| 20:00-21:59 | 0.5 | "50% lower in the evening" |
| 22:00-04:59 | 0.15-0.25 | Junk slots used in artificial exchanges |

**G, buyer and gauge:** long-haul wide-body 1.5; short-haul narrow-body 0.5; regional aircraft under 100 seats 0.25 (long-haul EBIT per slot at Heathrow is several times short-haul). A useful cross-check: observed prices equal roughly 2.5-3.5 years of the buyer's route-level operating profit (GBP 12-17m a year against a GBP 40m price).

**K, cycle:** 0.3 in a deep downturn (2009-12, 2020-21), 1.0 normal, 1.5-2.0 at peaks (2007-08, 2016-17, 2025).

Other parameters (**ESTIMATE** unless noted): one weekday-season unit = 1/28 of a daily pair, with summer worth about 60% and winter 40%; lease price 6-10% of value per year; portfolio discount 50-80% for blocks above 20 pairs or slots sold with a loss-making airline attached; remedy slots have zero resale value; collateral value about 50% of market value.

**Coordinated airports by IATA code with a constraint tier.** Status is from memory and should be checked against WASG Annex 12.7 each season; tiers are my **ESTIMATE**.

- **Tier S**: LHR, HND.
- **Tier A**: LGW, LCY, AMS, ORY, LIN, LIS, DCA, LGA, JFK, MEX, CGH, SYD, HKG, PEK, PVG, SHA, CAN, DEL, BOM, DXB, MNL, SGN.
- **Tier B**: STN, LTN, MAN, DUB, CDG, NCE, BRU, FRA, MUC, DUS, BER, ZRH, GVA, VIE, CPH, MAD, BCN, PMI, FCO, CIA, ATH, IST, SAW, TLV, JED, NRT, FUK, ICN, GMP, CJU, PKX, SZX, CTU, KMG, XIY, CKG, HGH, XMN, TPE, SIN, BKK, DMK, HKT, HAN, CGK, DPS, BLR, YYZ, GRU, SDU, BOG. US Level 2 airports with schedule caps behave like Tier B/C without tradable slots: EWR (B), ORD, LAX, SFO (C).
- **Tier C**: BHX, BRS, LYS, RTM, EIN, HAM, STR, INN, ARN, OSL, BGO, SVG, HEL, KEF, AGP, ALC, IBZ, MAH, VLC, SVQ, BIO, LPA, TFS, TFN, ACE, FUE, OPO, FAO, FNC, MXP, BGY, VCE, NAP, FLR, TRN, BLQ, CTA, PMO, CAG, HER, RHO, CFU, JTR, JMK, SKG, KGS, ZTH, CHQ, PRG, WAW, AYT, ADB, DLM, BJV, ESB, DWC, AUH, RUH, MED, DOH, JNB, CPT, DUR, MRU, MAA, HYD, CCU, CMB, DAC, KTM, KIX, PUS, TFU, NKG, WUH, CSX, TAO, TSN, DLC, SYX, HAK, URC, CGO, MFM, KUL, CNX, DAD, CEB, SUB, MEL, BNE, PER, AKL, LIM, HAV.

### (c) Airport and en-route charge formulas

**Airport charges per turnaround.** Let *I* be the airport's index in SDR per passenger (table in 5.1; regional defaults below) and C = 1.33 x I in USD. Using the Jacobs sample ratio of 0.80 t MTOW per passenger and a 25 / 75 split between aircraft-related and passenger-related charges (Frankfurt's actual split is 21-36% aircraft-related):

    turn_cost_USD = C x (0.31 x MTOW_tonnes + 0.75 x departing_passengers)

Check: at I = 43.5 (Frankfurt), an A320 with 146 passengers costs 57.9 x (22.8 + 109.5) = USD 7,660 and a 777-300ER with 301 passengers 57.9 x (109.0 + 225.8) = USD 19,380, against EUR 6,950 and EUR 17,600-20,200 (about USD 7,500 and USD 19,000-21,800) built from the 2026 tariff itself in 5.3.

Refinements: transfer passengers pay about 50-70% of the origin-destination passenger rate (Frankfurt 16.52 against 24.56-34.01; Changi 9 against 65.20); long-haul passengers pay 1.3-2 times short-haul where charges are differentiated (Frankfurt 1.4x, Heathrow about 2x); night movements at noise-sensitive airports multiply the aircraft-related part by 1.65-4; new-generation aircraft earn a 10-20% reduction at noise-banded airports (Heathrow's index falls 17% on a modern mix).

| Region | Default I (SDR per passenger) | Range |
|---|---|---|
| Heathrow | 62 | 56-68 |
| Australia / New Zealand gateways | 55 | 45-58 |
| Canada | 42 | 36-48 |
| US north-east and California gateways | 40 | 26-44 |
| Other US large hubs | use CPE: USD 12.8 median, 4-9 at low-cost hubs, 24-34 at ORD / BOS / SFO / LAX | |
| Germany, Switzerland, Benelux, Central Europe hubs | 38 | 30-50 |
| France, Italy, Iberia, Nordics | 27 | 12-37 |
| European secondary / low-cost airports | 8 (ESTIMATE), 1-4 net of incentives | |
| Russia | 53 | |
| Gulf | 18 | |
| East Asia | 20 | 14-25 (Singapore 34) |
| South-East Asia | 15 | 11-21 |
| South Asia | 16 | 14-18 |
| Africa | 16 | |
| Latin America | 20 (Mexico 40-46) | 15-46 |

For US airports model the two parts explicitly: landing fee = rate per 1,000 lb x MLW (table in 5.2: about 1.7-2.5 at cheap hubs, 4-7 typical, 9-12 at ORD / JFK / EWR, 20 at LGA), and terminal cost = (CPE x enplaned passengers) minus the landing fee.

**En-route charges.**

    Europe:   cost = sum over zones of unit_rate x (km in zone / 100) x sqrt(MTOW_t / 50)
              default unit rate EUR 70 (ESTIMATE of the traffic-weighted mean); by zone EUR 22-170 (table in 5.5)
              per km at EUR 70: A320 0.85, 737-800 0.88, 787-9 1.58, 777-300ER 1.86
    US:       domestic and US-landing flights 0 (ticket taxes fund ATC);
              pure overflights USD 0.333 per km en route, USD 0.143 per km oceanic, any aircraft size
    Elsewhere (ESTIMATE): USD 0.40 x sqrt(MTOW_t / 50) per km; double it for Russia or China long-haul corridors

Ticket taxes should be modelled separately from airport charges because they scale with fare (US 7.5%) or distance band and cabin (UK, France, Germany) and are borne by demand rather than paid per turn.

### (d) Delay cost per minute by aircraft size

Tactical (day-of-operation) delay cost is convex in delay length. A quadratic fitted to the EUROCONTROL / Westminster at-gate values (5, 15, 30 minutes; EUR, 2022 prices), cost = a x minutes + b x minutes^2 (fit is mine; valid to roughly 60-90 minutes):

| Class (reference type) | a | b | Cost of 15 min | Cost of 30 min | Average per minute at 30 min | Cost of 60 min (extrapolated) |
|---|---|---|---|---|---|---|
| Turboprop 70 seats (AT72) | 5.7 | 0.89 | 286 | 974 | 32 | 3,550 |
| Regional jet 100 seats (E190) | 5.1 | 1.35 | 380 | 1,366 | 46 | 5,170 |
| Narrow-body 150-180 seats (A320 / 737-800) | 7.1 | 2.16 | 594-641 | 2,162-2,305 | 72-77 | 8,200 |
| Large narrow-body (A321 / 757) | 6.9 | 2.7 | 689-736 | 2,566-2,720 | 86-91 | 10,100 |
| Wide-body 250-300 seats (A330-200; use for 787) | 16.2 | 4.15 | 1,176 | 4,218 | 141 | 15,900 |
| Large wide-body 350-400+ seats (747-400; use for 777-300ER / A350-1000) | 18.9 | 5.97 | 1,627 | 5,939 | 198 | 22,600 |

Multipliers by phase relative to at-gate for a 30-minute delay: taxi 1.2-1.3; airborne 1.5-1.8 (fuel). Network averages for quick use: EUR 100 per minute of ATFM delay; EUR 166 per minute at gate and EUR 212 airborne across all delay lengths; EUR 45 and EUR 89 for delays under 30 minutes. Strategic (schedule padding) minutes cost EUR 18 at gate, EUR 47 taxi, EUR 83 airborne.

US calibration: direct operating cost per block minute averaged USD 98-101 in 2023-2025 across the fleet (crew 37, fuel 29, maintenance 18, ownership 10, other 4 in 2025). **ESTIMATE** by size: regional jet USD 45-60, narrow-body 80-110, wide-body 180-280 airborne; at the gate use about half (no fuel burn). Add passenger time at USD 47 per hour if modelling welfare or goodwill.

Cancellation on the day (EUR, 2022 prices): 50 seats 6,800; 120 seats 16,600; 180-189 seats 18,600-25,700; 250 seats 85,600; 400 seats 123,900.

---

## Limits of this research

- The shared web-search budget for the session was exhausted during the charges work. Items tagged [R] could not be re-checked; the most important are: the exact Heathrow tariff lines; slot carrying values in the United, American and JetBlue 10-Ks; the JetBlue USD 75m price; ground-delay-program counts by airport; UK APD rates before April 2026; the EUROCONTROL charge formula wording; EU waiver regulation numbers; non-European en-route rates.
- No per-airport GDP statistics, no verified Haneda, Narita, Sydney, Dubai, Delhi or Sao Paulo tariff lines and no verified slot lease prices were obtained; the report uses the Jacobs index and flagged estimates instead.
- Slot prices are a thin and biased sample (2.1 publicly priced Heathrow pairs a year); the tier values for airports without a cash market are shadow values for game balance, not observations.

## Sources

Airport slot rules
- IATA, Airport Slots Fact Sheet (June 2026): https://www.iata.org/en/iata-repository/pressroom/fact-sheets/fact-sheet-airport-slots/
- IATA policy page on slots: https://www.iata.org/en/policy/slots ; coordinated airports: https://www.iata.org/en/programs/ops-infra/slots/coordinated-airports
- Worldwide Airport Slot Guidelines, Edition 4: https://www.iata.org/contentassets/4ede2aabfcc14a55919e468054d714fe/wasg-edition-4-english-version.pdf
- IATA Calendar of Coordination Activities (Nov 2026): https://www.iata.org/contentassets/c1d7626d7175462ab0fc527c9e2937ce/calendar-coordination-activities.pdf
- ACI EUROPE position paper on slots (March 2025): https://www.aci-europe.org/component/attachments/attachments.html?id=3377
- Wikipedia, "Landing slot": https://en.wikipedia.org/wiki/Landing_slot
- Regulation (EU) 2022/2038: https://eur-lex.europa.eu/eli/reg/2022/2038/oj
- UK slot alleviation, summer 2022: https://www.gov.uk/government/news/government-supports-aviation-sector-recovery-with-airport-slot-alleviation-for-summer-2022 ; explanatory memoranda https://legislation.gov.uk/uksi/2022/368/pdfs/uksiem_20220368_en.pdf and https://legislation.gov.uk/uksi/2022/822/pdfs/uksiem_20220822_en.pdf
- UK slot reform consultation: https://gov.uk/government/consultations/airport-slot-allocation-system-reform ; CMA response https://gov.uk/government/publications/cma-response-to-airport-slot-allocation-system-reform-consultation ; https://www.dacbeachcroft.com/en/what-we-think/all-change-for-uk-airport-slot
- Ghost flights: https://www.bloomberg.com/news/articles/2022-01-13/eu-defends-airline-slot-rules-over-lufthansa-ghost-flights-row ; https://www.vice.com/en/article/lufthansa-are-flying-18000-ghost-flights-over-europe-just-to-keep-their-slots/
- FAA orders: JFK https://www.federalregister.gov/documents/2026/06/23/2026-12591/operating-limitations-at-john-f-kennedy-international-airport ; LGA https://www.federalregister.gov/documents/2026/06/23/2026-12592/operating-limitations-at-new-york-laguardia-airport ; staffing relief https://www.govinfo.gov/content/pkg/FR-2025-07-25/html/2025-14100.htm and https://www.transportation.gov/regulations/federal-register-documents/2026-12593 ; summary https://avweb.com/aviation-news/faa-extends-northeast-atc-staffing-relief/
- Newark limits: https://www.faa.gov/newsroom/faa-proposes-extend-order-limiting-operations-newark-liberty-international-airport ; https://centreforaviation.com/news/us-faa-extends-arrivals-and-departures-limits-at-newark-airport-until-24-oct-2026-1333029
- High Density Rule history and AIR-21: https://www.govinfo.gov/content/pkg/FR-2008-04-17/html/E8-8308.htm
- DCA perimeter and exemptions: https://www.everycrsreport.com/reports/IN12504.html ; https://www.flyreagan.com/dca/dca-reagan-national-slot-perimeter-rules ; https://enotrans.org/article/a-history-of-the-perimeter-rule-at-washington-reagan-national-airport/
- GAO-12-902: https://www.gao.gov/products/gao-12-902
- Haneda allocations: https://www.travelvoice.jp/english/ministry-of-transport-decides-to-allocate-50-additional-international-daytime-slots-a-day-of-haneda-airport-to-9-countries ; https://japantimes.co.jp/news/2013/10/02/business/ana-lands-11-haneda-slots-jal-is-given-five
- China: https://aviationweek.com/air-transport/airports-networks/china-gambles-slot-auctions ; https://amp.scmp.com/business/article/1895985/hammer-set-fall-chinas-first-airport-slot-auction ; https://yicaiglobal.com/news/caac-new-measures-to-manage-flight-slots-dole-them-out-more-fairly

Slot values and transactions
- IAG, "Slot Asset Values Exist Without Excess Airline Returns" (19 May 2026), on the CAA website: https://www.caa.co.uk/media/ux5dbjdu/20260519_paper_on_slots_v1.pdf
- Aviation Strategy, "bmi and LHR slot valuation" (June 2008): https://aviationstrategy.aero/newsletter/Jun-2008/0/bmi-and-LHR-slot-valuation
- OAG, "The Worth of Heathrow's Slots" (Feb 2023): https://www.oag.com/blog/heathrows-slots-europes-most-valuable
- CAPA, "Heathrow Airport's slot machine": https://centreforaviation.com/analysis/reports/heathrow-airports-slot-machine-hitting-the-jackpot-again-108646
- Aviation Week, "Economic Downturn Drives Down Slot Prices": https://aviationweek.com/air-transport/airports-networks/world-routes-economic-downturn-drives-down-slot-prices
- Oman Air / Kenya Airways: https://travelweekly.co.uk/all-content/oman-air-pays-record-price-for-heathrow-slots ; https://www.businesstraveller.com/news/2016/03/02/kenya-airways-sells-its-only-heathrow-slot/
- SAS: https://www.sasgroup.net/newsroom/press-releases/2017/sas-transfers-two-landing-and-take-off-rights-slot-pairs-at-london-heathrow-and-generates-a-positive-earnings-impact-of-musd-75/ ; https://aviationweek.com/air-transport/airports-networks/sas-raises-75-million-heathrow-slot-sale
- Croatia Airlines / Delta: https://www.businesstraveller.com/business-travel/2017/01/12/delta-pays-croatia-airlines-19-5-million-heathrow-slots/ ; https://www.ch-aviation.com/news/53993-croatia-airlines-to-lease-back-five-lhr-slots-sold-to-delta
- Jet Airways / Etihad: https://www.business-standard.com/article/companies/jet-airways-sells-heathrow-slots-to-etihad-for-70-mn-113022700258_1.html ; Cyprus Airways: https://www.ch-aviation.com/news/28858-cyprus-airways-sells-off-last-heathrow-slot-to-move-to-stansted
- IAG / bmi: https://www.bloomberg.com/news/articles/2011-12-22/ba-trumps-virgin-to-win-heathrow-slots-with-deal-buying-bmi-from-lufthansa ; https://aviationweek.com/air-transport/airports-networks/virgin-wins-heathrow-slots-bmi-takeover-deal
- Virgin Atlantic financings: https://globalcapital.com/securitization/article/28mxzybsnyarwpki6ij9c/virgin-atlantic-brings-first-ever-landing-slot-abs-deal ; https://aviationweek.com/air-transport/virgin-atlantic-uses-heathrow-slots-bond-collateral
- ACL slot trading: https://www.acl-uk.org/slot-trading/
- Delta / US Airways and the 2011 auction: https://ir.delta.com/news/news-details/2011/Delta-US-Airways-Announce-New-Agreement-to-Transfer-Flying-Rights-in-New-York-and-Washington-DC/default.aspx ; https://www.transportation.gov/briefing-room/jetblue-westjet-gain-slots-laguardia-reagan-national-airports ; https://www.bloomberg.com/news/articles/2011-11-23/laguardia-reagan-flight-slots-fetch-100-million-in-bids
- American / US Airways decree and proceeds: https://www.justice.gov/atr/case-document/file/514511/dl ; https://www.cbsnews.com/dfw/news/us-american-airlines-got-425m-for-landing-rights/ ; https://www.fool.com/investing/general/2014/03/11/american-airlines-gets-a-windfall-today-but-compet.aspx
- United / Delta Newark: https://www.courthousenews.com/feds-sue-to-stop-united-newark-airport-deal/ ; https://money.cnn.com/2016/04/06/news/companies/united-delta-newark-airport/index.html
- Delta FY2024 10-K: https://www.sec.gov/Archives/edgar/data/27904/000002790425000004/dal-20241231.htm
- Thomas Cook: https://www.headforpoints.com/2019/11/09/thomas-cook-slots-sold/ ; Monarch: https://www.mondaq.com/uk/aviation/672316/english-court-of-appeal-restores-insolvent-airlines-valuable-slots and https://natlawreview.com/article/fight-over-runway-ii-monarch-successful-appeal ; Flybe: https://headforpoints.com/2023/02/18/british-airways-gains-86-weekly-flybe-heathrow-slots and https://www.ch-aviation.com/portal/news/126133-flybe-unrescuable-with-826mn-debts-say-administrators
- Air Berlin: https://ec.europa.eu/competition/mergers/cases/decisions/m8672_673_5.pdf ; https://cms.law/en/bel/legal-updates/transfer-of-air-berlin-assets-easyjet-cleared-right-away-lufthansa-cleared-after-making-commitments
- Lufthansa / ITA: https://www.easyjet.com/en/news/story/easyjet-confirms-expansion-plans-in-italy-with-new-bases-at-milan-linate-and-rome-fiumicino-opening-spring-2025 ; https://www.ch-aviation.com/news/147614-eu-approves-remedy-takers-for-lufthansa-ita-takeover
- Korean Air / Asiana: https://aviationweek.com/air-transport/safety-ops-regulation/kftc-begins-korean-air-asiana-slot-transfers-10-routes ; https://www.ch-aviation.com/portal/news/125105-uk-greenlights-korean-air-asiana-merger

ATFM
- EUROCONTROL ATFCM Users Manual (Wave 2.6): https://www.eurocontrol.int/archive_download/all/node/10578
- EUROCONTROL Performance Review Report 2024: https://www.eurocontrol.int/publication/performance-review-report-prr-2024 (copy read: https://www.atceuc.org/uploads/docs/eurocontrol-performance-review-report-2024.pdf)
- EUROCONTROL Standard Inputs for Economic Analyses, release 10.0.2: https://www.ansperformance.eu/economics/cba/standard-inputs/ (chapters "Cost of delay", "Cancellation cost")
- University of Westminster delay cost study: https://westminsterresearch.westminster.ac.uk/item/q2614
- Vossen, Ball, Montebello, "Dynamic Airline Slot Exchange during Ground Delay Programs": https://nextor.umd.edu/files/2019/02/2004_GAIFP_Thomas_Vossen-1xsiaxw.pdf ; FAA facility order on traffic management: https://www.faa.gov/air_traffic/publications/atpubs/foa_html/chap18_section_10.html
- NBAA, Ground Delay Program: https://nbaa.org/aircraft-operations/airspace/tfm/tools-used-for-traffic-flow-management/ground-delay-program-gdp/
- NEXTOR Total Delay Impact Study: https://rosap.ntl.bts.gov/view/dot/6234
- A4A, US passenger carrier delay costs: https://airlines.org/dataset/u-s-passenger-carrier-delay-costs/

Charges and taxes
- Jacobs, Review of Airport Charges bespoke analysis (29 May 2025): https://www.caa.co.uk/media/nibiyl1c/a3-37-jacobs_review-of-airport-charges-bespoke-analysis_may-2025.pdf
- Port Authority of NY & NJ schedule of charges (Sept 2025): https://www.panynj.gov/content/dam/airports/pdfs/Airport%20Schedule%20of%20Charges_SEPT%202025.pdf
- DWU Consulting compilations: https://dwuconsulting.com/info/data/airline-rates ; https://dwuconsulting.com/info/data/cpe ; Atlanta FY2026: https://www.atl.com/wp-content/uploads/2025/07/FY-2026-Rates-and-Charges-Summary.pdf
- Fraport charges 2026: https://www.fraport.com/en/business-areas/operations/airport-charges.html
- CAA on Heathrow: https://caa.co.uk/newsroom/news/regulator-decides-on-changes-to-heathrow-airport-limited-s-airline-charges-in-response-to-cma-appeal ; https://www.caa.co.uk/newsroom/news/aviation-regulator-publishes-initial-proposals-for-heathrow-h8-price-cap/ ; https://www.caa.co.uk/media/upcoupbx/economic-regulation-of-heathrow-airport-h8-initial-proposals-summary.pdf
- Schiphol: https://flightglobal.com/airlines/higher-fares-inevitable-as-klm-slams-schiphol-charges-increase/160546.article
- Changi: https://www.caas.gov.sg/resources/media-and-publication/newsroom/changi-airport-to-invest-s3-billion-over-next-six-years-to-strengthen-singapores/ ; https://milelion.com/2024/11/08/changi-airport-plans-21-hike-in-passenger-fees-by-2030-to-fund-s3-billion-investment/
- EUROCONTROL adjusted unit rates, November 2026: https://www.eurocontrol.int/publication/monthly-adjusted-unit-rates-november-2026
- FAA overflight fees, 14 CFR 187.53: https://www.law.cornell.edu/cfr/text/14/187.53
- Russian overflight payments: https://www.themoscowtimes.com/2010/10/28/europe-frets-about-flight-fees-a2597 ; https://themoscowtimes.com/archive/aeroflot-retains-170m-in-overflight-fees ; https://www.flightglobal.com/eu-russian-talks-on-charges-stall/69745.article
- A4A, government-imposed taxes on air transportation: https://www.airlines.org/dataset/government-imposed-taxes-on-air-transportation/
- UK APD rates: https://www.gov.uk/guidance/rates-and-allowances-for-air-passenger-duty
- German air tax: https://de.wikipedia.org/wiki/Luftverkehrabgabe ; French TSBA: https://fr.wikipedia.org/wiki/Taxe_de_solidarit%C3%A9_sur_les_billets_d%27avion ; Dutch tax: https://nl.wikipedia.org/wiki/Vliegbelasting_in_Nederland ; overview: https://en.wikipedia.org/wiki/Air_passenger_tax
- Ground handling KPI source: https://pulserevops.com/industry-kpis/ik786

Airport competition
- Commission Decision 2004/393/EC (Ryanair / Charleroi): https://lexaris.de/book/version/documentflat/head/2038442 ; outcome: https://www.irishtimes.com/business/ryanair-to-repay-charleroi-subsidies-1.1164062 ; https://www.rte.ie/news/2008/1217/111772-ryanair ; later decision https://www.legislation.gov.uk/eudn/2016/2069/body/data.htm
- Essential Air Service: https://www.transportation.gov/policy/aviation-policy/small-community-rural-air-service/essential-air-service ; https://en.wikipedia.org/wiki/Essential_Air_Service ; https://www.kucb.org/regional/2026-08-06/trump-administration-threatens-deep-cuts-to-program-that-funds-air-travel-in-rural-alaska

