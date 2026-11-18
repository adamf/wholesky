# Aircraft turnaround times and minimum connecting times (MCT)

Research notes for parameterising an airline simulator and a reservation gateway. Compiled 2026-11-18.

## How to read this document

Every number carries an evidence tag. The tags matter here more than usual, because the session's shared web-search budget ran out part-way through and most airport-specific MCT values could not be looked up. What was retrieved is solid (manufacturer documents, the IATA chapter itself, EUROCONTROL, BTS, MIT, one airline's published station table); what was not is marked as an estimate.

| Tag | Meaning |
|---|---|
| **[OEM]** | Read this session from the manufacturer's airport-planning document (text or Gantt chart). |
| **[PUB]** | Published value retrieved this session; source named. "Secondary" means a blog, guide or wiki rather than the airline, airport or IATA. "Search extract" means the publisher's page blocked direct reading and the value was seen only in a search-engine snippet. |
| **[EX]** | Value that appears in an IATA SSIM / MCT guide worked example. Real station and carriers, but illustrative and possibly dated. |
| **[REC]** | Recalled industry figure, not re-verified this session. Treat as an ESTIMATE. |
| **[EST]** | ESTIMATE inferred from airport class and layout, or scaled from a similar aircraft. |

Authoritative MCT data is the IATA MCT file distributed by OAG and Cirium. Nothing below replaces it; Table 2 is a set of defaults for when that file is not available.

---

## Table 1. Turnaround minutes by aircraft type and carrier model

"Min" is the minimum ground time a scheduler would allow (the hard constraint in aircraft routing). "Typ" is a typical scheduled turn at a non-hub station; hub turns are longer because the aircraft waits for its outbound bank. OEM columns are the manufacturer's planning figures. The three carrier-model columns are modelling defaults built from the OEM figures and the airline evidence in section 3; they are estimates unless a source is named.

| Aircraft (typical seats) | OEM full-service turn | OEM minimum / transit turn | ULCC / LCC min / typ | Network short-haul min / typ | Long-haul or international min / typ |
|---|---|---|---|---|---|
| ATR 72-600 (68-78) | no public chart retrieved; 20-25 [REC] | — | 20 / 25 | 25 / 30 | 30 / 45 |
| Dash 8-400 (74-90) | not retrieved; 20-25 [REC] | — | 20 / 25 | 25 / 30 | 30 / 45 |
| CRJ700/900, E175 (65-88) | about 16-18 by the Embraer method [EST] | — | 25 / 30 | 30 / 40 | 40 / 55 |
| E190 / E195, E2 (96-146) | 18 (E190), about 19 (E195-E2), including 2 min pushback [OEM] | not given ("not applicable") | 25 / 30 | 30 / 40 | 45 / 60 |
| A220-100/-300 (110-150) | not retrieved; 30-35 [EST] | — | 25 / 35 | 35 / 45 | 50 / 65 |
| A319 (124-156) | 37 (124 pax, one bridge) [OEM] | 21 (156 pax, two stairs) [OEM] | 25 / 30 | 35 / 45 | 50 / 65 |
| A320 (150-186) | 44 (150 pax, one bridge) [OEM] | 22 (180 pax, two stairs, no fuel) [OEM] | 25 / 30 | 40 / 50 | 55 / 75 |
| A321 (185-244) | 52 (185 pax, one bridge) [OEM] | 23 (220 pax, two stairs) [OEM] | 30 / 40 | 45 / 60 | 60 / 90 |
| 737-700 (126-149) | 35.4 (140 pax) [OEM] | thru-stop chart exists, not read | 25 / 35 | 35 / 45 | 50 / 65 |
| 737-800, 737 MAX 8 (160-197) | 37.5 (737-800, 162 pax); about 40 (737-8) [OEM] | 16.5 (thru-stop, 60% exchange, no servicing) [OEM] | 25 / 30-35 | 40 / 50 | 55 / 75 |
| 757-200 (176-239) | about 39 (186 pax) [OEM] | — | 40 / 50 | 45 / 60 | 70 / 90 |
| 767-300ER (211-290) | about 55 (218 pax, 3-class); 767-200 about 42 [OEM] | — | 60 / 75 | 55 / 70 | 75 / 105 |
| A330-200/-300 (250-440) | 51 (-200, 293 pax); 55 (-300, 332 pax) [OEM] | 32 / 34 (50% exchange, 30% fuel) [OEM] | 60 / 75 | 60 / 75 | 90 / 120 |
| 787-8 / -9 (240-360) | about 41.5 (787-8, 274 pax); about 51 (787-9, 360 pax) [OEM] | about 29 (787-9 en route, 80% exchange, no servicing) [OEM] | 60 / 75 | 55 / 70 | 90 / 120 |
| A350-900 (300-369) | 62 (315 pax, two bridges) [OEM] | 34 (50% exchange) [OEM] | 70 / 90 | 60 / 75 | 90 / 120 |
| 777-300ER (350-470) | about 62 (441 pax, two doors); 64 without the fuelling service bulletin [OEM] | about 35 (en route, 50% exchange) [OEM] | 70 / 90 | 60 / 75 | 90 / 135 |
| 747-400 / 747-8 (364-524) | 108.5 (747-8, 467 pax, two doors) [OEM] | — | — | 75 / 90 | 120 / 180 |
| A380 (480-615) | 89 (bridges to main and upper deck, upper-deck catering trucks); 139 (all servicing via main deck) [OEM] | — | — | — | 110 / 150-180 |

Calibration points for the carrier-model columns:

- ECAC-wide scheduled turnaround in 2022 [PUB, EUROCONTROL Standard Inputs, from CODA]: medium wake category 10th / 50th / 90th percentile = 25 / 45 / 80 min; heavy = 60 / 90 / 150 min. Mean scheduled 50 (medium) and 119 (heavy); mean actual 59 and 134. The LCC column sits at the 10th percentile, the network column at the median.
- Ryanair's standard is 25 min [PUB, secondary]; a 2018 study of 509 Ryanair turns at Valencia measured a median of 30 and a mean of 40. easyJet's standard for the A319/A320 is 25 min using both doors [PUB, search snippet of an easyJet document]. AirAsia targets 25 (group) and measured 29-30 [PUB, secondary].
- Southwest: 10-minute turn from May 1972 (three aircraft covering an hourly Dallas-Houston pattern after the fourth 737 was sold; 25 min had been planned) [PUB, Wikipedia, itself flagged "better source needed"]. Its 1978 annual report said going from 10 to 20 minutes would have needed two more aircraft. Adjusted domestic average 49 min in the 12 months to May 2024, target 44 by late 2025 [PUB, secondary citing the September 2024 investor day]. An older press figure is "about 35" [PUB, The Points Guy]. For a Southwest-like carrier use 737-700 35-40 and 737-800/MAX 8 45-50, not the European LCC values.
- Observed LCC turns in Germany (2008, A319 and 737-800, mostly two stairs) averaged 41.3 min with a minimum of 23 [PUB, HAW Hamburg thesis].
- Press ranges: LCC narrowbody 25-30, legacy narrowbody 45-60, regional jet 30-45, widebody long-haul 120-180 [PUB, secondary].

### Table 1b. Component times in the OEM charts

These are the bars in each manufacturer's Gantt chart, at the reference load stated. "Avail." means the task is fitted into whatever time the critical path leaves. Use the rates in Table 1c to rescale to a different seat count.

| Aircraft | Reference load | Deplane | Clean / cabin service | Catering | Fuel | Board | Critical path | Total |
|---|---|---|---|---|---|---|---|---|
| E190 | 100% LF, front door only | 4.1 | 6.1 | 7.0 fwd and 7.0 aft, in parallel | 11.7 (80% of tanks at 290 US gal/min) | 6.1 | passengers | 18 incl. 2 min push/start |
| E195-E2 | 100% LF, front door only | 5.6 | 6.1 | 7.0 and 7.0, parallel | 14.5 | 7.5 | passengers | about 19 |
| A319 | 124 pax, 1 bridge | 6.2 | avail. | not extracted | not extracted | 10.3 + 2 | passengers and catering | 37 |
| A320 | 150 pax, 1 bridge | 7.5 | avail. | 11 trolleys x 1.2 = 13.2, plus 5.5 positioning and 2 driving between doors | 16 + 2.5 connect (20,000 L at 1,250 L/min average) | 12.5 + 2 | deplane, catering, board | 44 |
| A321 | 185 pax, 1 bridge | 9.3 | avail. | 14 trolleys = 16.8, plus positioning | 16 + 2.5 | 15.4 + 2 | deplane, catering, board | 52 |
| A320 minimum | 180 pax, 2 stairs | 5.0 | avail. | as required | none | 7.5 + 2 | passengers and containers | 22 |
| 737-700 | 140 pax, 1 door | 7.8 | 13.0 | 3.0 fwd, 7.0 aft, sequential | 16.2 (22,700 L) | 11.7 | deplane, galleys, board | 35.4 |
| 737-800 | 162 pax, 1 door | 9.0 | 11.0 | 4.0 fwd, 5.0 aft, sequential | 15.2 (26,000 L) | 13.5 | deplane, galleys, board | 37.5 |
| 737-8 (MAX) | 100% exchange, 1 door, "light cleaning" | 9.0 | 10.8 | 5.5 and 6.5 | 15.0 (assumed) | 13.5 | passengers | about 40 |
| 737-800 thru-stop | 60% exchange | 5.5 | none | none | none | 8.0 | passengers | 16.5 |
| 757-200 | 186 pax | 10.0 | 10.0 | 20.0 | 15.0 (30,950 L at 2,063 L/min) | 21.0 (at 9 pax/min) | passengers | about 39 |
| 767-300ER | 218 pax, 1 door | 5.4 | 38 avail. | 23.0 fwd then 12.0 aft | 24 (55,600 L); 37 for 83,800 L, at 30 psi | 8.7 | galley service | about 55 |
| A330-300 | 332 pax, 2 bridges | 6.6 | avail. | 39 trolleys x 1.5 across three trucks; longest door 27 + 8 positioning | 90,000 L, two nozzles | 11.1 + 4 | catering | 55 |
| 787-8 | 274 pax, 1 door | 6.9 | 19.5 | 6.0 / 10.5 / 15.0 at three galleys | 35 (112,800 L, four nozzles) | 11.7 | forward hold: 16 off + 16 on | about 41.5 |
| 787-9 | 360 pax, 1 door | 9.0 | 17.0 | 10.0 / 4.0 / 18.0 | 35 (112,250 L) | 20.0 | forward hold: 20 off + 20 on | about 51 |
| A350-900 | 315 pax, 2 bridges | 6.3 | avail. | 42 trolleys; door R4 25 x 1.5 = 37.5 + 8 positioning | 81,000 L, two nozzles | 10.5 + 4 | catering at R4 | 62 |
| 777-300ER | 441 pax, 2 doors | 7.4 | 29.4 avail. | 7.5 / 6.0 / 10.0 / 10.0 at four doors | 42 (167,300 L); 56 without the fuelling modification | 21.3 | forward hold: 24 off + 24 on | about 62 |
| 747-8 | 467 pax, 2 doors | 7.8 | 87 avail. | 10 fwd, 87 mid and upper, 28 aft (84 carts, three trucks) | 44 (223,500 L) | 11.7 | mid and upper galley | 108.5 |
| A380 | 555 pax, bridges at M2 and U1 | 14.2 | avail. | 78 trolleys; 1.5 min each direct, 2.0 via lift | 242,700 L (75% of capacity), four nozzles at 40 psi | 23.7 + 4 | catering | 89, or 139 if upper deck is served through main-deck lifts |

All [OEM]. Boeing figures are read from charts and are good to about half a minute.

### Table 1c. Process rates

| Process | Rate | Basis |
|---|---|---|
| Deplane, single-aisle, one door | 18 pax/min (Boeing 737, 757), 20 pax/min (Airbus), 24 pax/min (Embraer), 22 (A318) | [OEM] |
| Board, single-aisle, one door | 12 pax/min (737, A320 family), 9 pax/min (757-200), 16 (Embraer), 18 (A318) | [OEM] |
| Single-aisle with two doors | Airbus: 18 off and 12 on per door. Observed 2008: 33.6 off and about 20 on in total, so roughly 17 and 10 per door | [OEM]; [PUB, HAW Hamburg] |
| Twin-aisle | Boeing 767 and 787: 40 off, 25 on through one door. 777-300ER: 60 off, 40 on through two doors. Airbus: 25 off, 15 on per door | [OEM] |
| Boarding allowance | Last passenger seated and headcount: +2 min narrowbody, +4 min widebody (Airbus) | [OEM] |
| Bridge or stairs | on 2 min, off 1 to 1.5 min. Observed stairs: 52 s on, 31 s off | [OEM]; [PUB] |
| Observed boarding, one bridge | 158 passengers in 14 min at Gatwick, 2019 trial, 11.3 pax/min | [PUB, secondary] |
| Boarding procedure effects | Two doors up to 30% faster; one-bag rule 5-15% faster; seat-by-seat sequencing up to 35% faster than random (simulation) | [PUB, Schultz, WSC 2017] |
| Catering trolley exchange | 1.2 min per full-size trolley (narrowbody), 1.5 (widebody), 2.0 via A380 lift; truck positioning 2 + 1.5 (narrow), 5 + 3 (wide) | [OEM] |
| Fuel flow | A320 family 1,250 L/min average through one hose (peak 1,475); E-Jet 1,100 L/min; 757 2,063 L/min; 787 and 777 about 3,500-4,300 L/min through four nozzles; 747-8 about 5,400 L/min. Add 2.5-3 min each side for connection | [OEM]; peak figure [PUB, HAW Hamburg citing Airbus] |
| Bulk bags, per belt loader | 15 bags/min off, 10 bags/min on; 1.0-1.25 bags per passenger | [OEM, Boeing]; Embraer uses 1.2 bags per passenger |
| Containers | Narrowbody 1.5 min each way. Widebody LD3 1.2 off, 1.4 on; pallet 2.4 off, 2.8 on; bulk 9.2 min/t off, 10.5 min/t on | [OEM, Airbus] |
| Water and lavatory | Potable 60 L/min; lavatory truck 38 L/min; 5-10 min each (narrowbody), 10-14 (widebody) | [OEM]; [PUB] |

---

## Table 2. Station-standard MCTs by airport (minutes)

Read the basis column before using a row. Only rows tagged [PUB] or [EX] were retrieved this session. The largest single source is Ethiopian Airlines' public "minimum connecting time" page, which reproduces the industry table for the stations it serves; where its row equals an IATA global default (old 20/60/60/60 or current 30/60/90/90) it carries no station-specific information and is not used.

### Americas

| Airport | DD | DI | ID | II | Terminal pairs and carrier exceptions | Basis |
|---|---|---|---|---|---|---|
| ATL | 55 | 60 | 90 | 90 | Delta online DD 35 [REC]. All concourses airside by train at 2-min headway. | ESTIMATE [REC] |
| ORD | 50 | 75 | 90 | 90 | American DD 40, United DD 35-40 [REC]. International arrivals at T5 need a landside transfer. | ESTIMATE [REC] |
| DFW | 50 | 60 | 90 | 90 | American DD 40 and II about 70 [PUB, secondary: airlinefyi]. Skylink ride averages 5 min, longest 9. | ESTIMATE [REC]; exception secondary |
| DEN | 50 | 60 | 90 | 90 | United DD 35-40 [REC]. | ESTIMATE |
| LAX | 70 | 90 | 120 | 120 | American online DD 40, and 45 in summer 2017 [EX]. | [PUB] Ethiopian station table; matches an ExpertFlyer reading (Travelsort) |
| JFK | 60 | 75 | 105 | 120 | Delta online DI 45 and ID 45; Virgin Atlantic to a Delta-operated VS codeshare ID 60 [EX]. Inter-terminal by landside train. | [PUB, secondary] ExpertFlyer reading via Travelsort, about 2014 |
| EWR | 45 | 60 | 90 | 90 | United DD 30-40 [REC]. | ESTIMATE |
| MIA | 45 | 60 | 90 | 90 | DI standard 60, American exception 55 [PUB, OAG]. BA to AA ID 90; AA-marketed BA-operated to AA ID 100 [EX]. | DI [PUB]; rest ESTIMATE |
| IAH | 45 | 60 | 90 | 90 | United DD 30-35 [REC]. | ESTIMATE |
| IAD | 45 | 45 | 90 | 90 | — | [PUB] Ethiopian station table |
| SFO | 50 | 60 | 90 | 90 | — | ESTIMATE |
| SEA | 45 | 60 | 90 | 90 | Alaska and Delta DD about 40 [REC]. | ESTIMATE |
| CLT | 40 | 55 | 85 | 85 | American DD 25-30 [REC, low confidence]. | ESTIMATE |
| SLC | 40 | 60 | 90 | 90 | — | DD [PUB, Travelport help example]; rest ESTIMATE |
| YYZ | 40 | 60 | 75 | 60 | Air Canada files by sector (domestic, transborder, international); values not retrieved. The Ethiopian table shows only the old IATA default. | ESTIMATE |
| MEX | 60 | 90 | 120 | 90 | T1 to T2 is landside: allow 150 or more. | ESTIMATE |
| GRU | 60 | 60 | 90 | 60 | — | [PUB] Ethiopian station table |
| BOG | 45 | 60 | 75 | 60 | — | ESTIMATE |

### Europe

| Airport | DD | DI | ID | II | Terminal pairs and carrier exceptions | Basis |
|---|---|---|---|---|---|---|
| LHR | 60 | 60 | 60 | 60 | Same terminal 60 (T2, T3, T5); T4 to T4 45-60; T2 to T3 75; T5 to T3 90 (one secondary source says 75); T4 to any other 90 [REC]. Same-terminal II 60 [PUB, secondary]. In IATA examples: AA or BA codeshare combinations II 105-120; United and Star partners in T2, II 60 [EX]. Heathrow's own pages give no numbers. | ESTIMATE [REC] |
| LGW | 45 | 60 | 60 | 60 | Airport guidance to passengers: same terminal 60, North to South 90 [PUB, Gatwick]. BA online about 45 [REC]. | ESTIMATE; guidance [PUB] |
| CDG | 60 | 60 | 60 | 60 | Air France within 2E and 2F: 60 on all four [PUB, secondary, ExpertFlyer reading]. 2C to 2E for a Delta/Air France codeshare: II 90 [EX]. 2G to 2E/2F about 90; T1 or T3 to T2 120-150 [EST]. | [PUB, secondary] for AF; rest ESTIMATE |
| AMS | 40 | 50 | 50 | 50 | Station standard II 50 is stated in SSIM 8.10.7 [EX]. Schengen to Schengen 40 [REC]. KLM to KLM II 55 and KLM to Lufthansa on an A380 II 90 appear in examples [EX]. One terminal. | II [EX]; rest [REC] |
| FRA | 45 | 45 | 45 | 45 | Lufthansa 45 [PUB, secondary]. United codeshares on Lufthansa or Air Canada II 60; from the US to Schengen 45 [EX]. T1 to T2 airside train 2-3 min. | [PUB] Ethiopian station table |
| MUC | 30 | 30 | 30 | 30 | Within Terminal 2 and its satellite 30; Terminal 1 35; T1 to T2 45 [REC]. A secondary source says T1 to T2 is 50-60 [PUB, Wikivoyage]. | ESTIMATE [REC] |
| ZRH | 40 | 40 | 40 | 40 | — | ESTIMATE [REC] |
| VIE | 30 | 30 | 30 | 30 | Austrian and Star online 25 [REC]. | [PUB] Ethiopian station table |
| MAD | 45 | 60 | 60 | 60 | The Ethiopian table shows 120 / 60 / 60 / 60, where the 120 probably reflects a landside change between T1-T3 and T4. Iberia within T4 and T4S: about 40-45 Schengen, 60 otherwise [REC]. American to Iberia ID 95-100 [EX]. | Partly [PUB]; DD ESTIMATE |
| FCO | 45 | 60 | 60 | 45 | — | [PUB] Ethiopian station table |
| IST | 45 | 75 | 90 | 60 | Turkish II 60 [REC]. A secondary source says "MCT is 75 minutes" without a status pair [PUB, Wikivoyage]. | ESTIMATE |
| HEL | 35 | 35 | 35 | 35 | "Minimum transit time of 35 minutes" [PUB, Wikipedia]. Schengen to non-Schengen may be 40-45 [REC]. | [PUB, secondary] |
| CPH | 30 | 40 | 40 | 45 | — | ESTIMATE |
| BRU | 20 | 50 | 50 | 50 | — | [PUB] Ethiopian station table |
| MXP | 60 | 60 | 60 | 60 | — | [PUB] Ethiopian station table |
| DUB | 45 | 45 | 45 | 45 | — | [PUB] Ethiopian station table |

### Middle East, Africa, Asia, Oceania

| Airport | DD | DI | ID | II | Terminal pairs and carrier exceptions | Basis |
|---|---|---|---|---|---|---|
| DOH | — | — | — | 45 | Qatar Airways online 45, or 60 onward to the US [secondary]. A Qatar Airways trade notice set transit MCT at 75 for travel up to 28 March 2026, as a temporary measure [PUB]. JAL to Qatar Airways about 60 [PUB, search extract of JAL's page]. Gates close 20 min before departure [PUB, airport]. | ESTIMATE [REC]; temporary 75 [PUB] |
| DXB | — | 60 | 60 | 75 | II 75 is the station-specific value. Guide sites give same terminal 60 and T1 to T3 75 [secondary]. T3 to T2 (Emirates to flydubai): 120 on one flydubai page and 4 hours on another [PUB, search extract; the two conflict]. Emirates online value not found. | II [PUB] Ethiopian station table |
| AUH | — | — | — | 60 | Etihad online 45-60 [REC]. | ESTIMATE |
| DEL | 90 | 180 | 180 | 90 | These look like legacy values from the split-terminal era. For a modern single-roof T3 connection use about 60 / 90 / 120 / 75 [EST]. | [PUB] Ethiopian station table, date unknown |
| BOM | 30 | 150 | 180 | 90 | Same caveat: predates the integrated T2. Modern T2 about 60 / 90 / 120 / 75 [EST]. | [PUB] Ethiopian station table, date unknown |
| SIN | — | — | — | 60 | JAL to other carriers about 60 [PUB, search extract of JAL's page]. Singapore Airlines online 50 [secondary]. T4 has no airside link to T1-T3; shuttle bus 10-18 min, so about 90 [EST]. | II 60 [PUB, extract]; rest ESTIMATE |
| HKG | — | — | — | 60 | Cathay Pacific II 60 [PUB, secondary, ExpertFlyer reading]; Cathay online 50 is also quoted [secondary]. | [PUB, secondary] |
| BKK | 75 | 75 | 75 | 75 | — | [PUB] Ethiopian station table |
| KUL | 60 | 60 | 60 | 60 | KLIA Terminal 1 to Terminal 2 is not an airside connection. | [PUB] Ethiopian station table |
| ICN | 40 | 70 | 90 | 70 | Asiana: T1 to T1 70, T2 to T2 70, T2 to T1 90 [PUB, search extract of Asiana's page]. Korean Air online II 45-60 [secondary]. Shuttle train between terminals 15-20 min. | II 70 [PUB, extract]; rest ESTIMATE |
| NRT | 50 | 45 | 75 | 60 | ANA online: DI 45, ID 75. ANA to or from another domestic carrier, domestic to domestic: at least 50 [PUB, search extract of ANA's page]. Same-terminal II 60 and T1 to T2 about 110 [REC]. | DI, ID, DD [PUB, extract], carrier values not station standards; II ESTIMATE |
| HND | 50 | 55-70 | 80 | 60 | ANA online: DI 55 when the international flight leaves from T2, 70 from T3; ID 80. Interline domestic to domestic at least 50 at HND and NRT, 30 at other Japanese airports [PUB, search extract of ANA's page]. Online DD about 30 [REC]. The airport publishes no MCT and refers to the airline [PUB]. | DI, ID, DD [PUB, extract], carrier values; II ESTIMATE |
| PEK | 90 | 90 | 90 | 90 | Air China online within T3 is shorter, about 60 for DD and II [EST]. | [PUB] Ethiopian station table |
| PVG | 90 | 120 | 120 | 120 | — | ESTIMATE |
| SYD | 30 | 90 | 75 | 60 | Qantas raised domestic to international from 60 to 90 from 21 August 2022 to cut bags missing international departures [PUB, trade press quoting Qantas; the article does not name the airports]. ID 75 [secondary]. Domestic (T2/T3) to international (T1) is across the airfield by bus, 15-25 min. | DI [PUB, secondary]; rest ESTIMATE |
| MEL | 30 | 90 | 75 | 60 | Same Qantas change reported for Melbourne [secondary, unconfirmed]. | ESTIMATE |
| JNB | 60 | 90 | 90 | 60 | — | [PUB] Ethiopian station table |
| ADD | 30 | 60 | 60 | 45 | Ethiopian online: 30 / 30 / 60 / 35. T1 domestic, T2 international. | [PUB] Ethiopian Airlines |
| CAI | 30 | 90 | 90 | 90 | — | [PUB] Ethiopian station table |
| NBO | 30 | 60 | 60 | 60 | — | [PUB] Ethiopian station table |
| CPT | 45 | 60 | 60 | 60 | — | [PUB] Ethiopian station table |
| LOS | 60 | 135 | 150 | 75 | — | [PUB] Ethiopian station table |
| JED | 75 | 150 | 150 | 90 | — | [PUB] Ethiopian station table |
| TLV | 20 | 90 | 90 | 90 | — | [PUB] Ethiopian station table; same values in a GDS reading on FlyerTalk |

A dash means the status pair does not occur (no domestic network).

### Inter-airport MCTs

| Pair | Minutes | Basis |
|---|---|---|
| Any pair with nothing filed | 240, all four status pairs | [PUB] SSIM 8.10.2 global default |
| LHR-LGW | 180 or more | ESTIMATE [REC]. Coach is 70-75 min [PUB, Heathrow]. |
| LHR-STN, LHR-LTN, LHR-LCY | 180-240 | ESTIMATE. Coach 80 and 55 min; rail to LCY 60-75 [PUB, Heathrow]. |
| JFK-LGA, JFK-EWR, LGA-EWR | 180-240 | ESTIMATE |
| CDG-ORY | 180 | ESTIMATE [REC] |
| NRT-HND | 180 | [PUB, search extract of ANA's transit page: "including approximate time required for transferring between airports"]. 210 is also quoted [REC]. |
| IST-SAW | 240 | ESTIMATE. Taxi about 58 min [PUB, Wikivoyage]. |
| PVG-SHA, PEK-PKX, DXB-DWC, GMP-ICN | 240 | ESTIMATE, IATA default |

### Checked-bag transfer limits

No airport or airline publishes an explicit bag cut-off that could be retrieved. The nearest thing to one is Qantas raising its domestic-to-international MCT from 60 to 90 minutes in August 2022 expressly because bags were missing international departures [PUB, trade press]: the passenger could make 60, the bag could not. Changi requires transit bags to be tagged through from origin [PUB, airport]. At Haneda an international-to-domestic passenger rechecks at a transfer counter in T3 and takes an airside bus to T1 or T2, but only during counter hours (JAL 05:30-11:00 and 13:30-19:45, ANA 05:30-19:30); outside them the transfer is landside [PUB, airport]. Working assumptions [REC/EST]: through-checked bags make an online same-terminal connection down to the passenger MCT at the efficient single-roof hubs, but the practical bag floor is about 30-35 min domestic and 45-60 min international; inter-terminal transfers need 60-90 min; any international arrival into the United States or Canada (unless precleared) means reclaim and recheck, which is why ID is 90 or more there. Published context: transfer mishandling is the largest single cause of mishandled bags, 39% in 2025 [PUB, SITA]. Ethiopian states that bags are transferred automatically on online connections "if the connection meets the MCT" [PUB].

---

## 3. Evidence behind Table 1

### 3.1 Manufacturer planning documents

Airbus "Aircraft Characteristics - Airport and Maintenance Planning" (chapter 5-2 full servicing, 5-3 outstation or minimum servicing) and Boeing "Airplane Characteristics for Airport Planning" (5.2 turnaround station, 5.3 en-route station) both give a Gantt chart with a critical path. Older Airbus revisions print the total as "TRT: nn min"; current revisions (A321, March 2022) show the chart only. Embraer's Airport Planning Manual has the same chart in section 5.3.

What the charts assume, and why airlines do not achieve them:

- 100% passenger and cargo exchange, all equipment and staff waiting at chocks-on, no passengers needing assistance, no crew change, no security search, no late bags, no slot wait.
- Cleaning is "performed in available time", so it never drives the total. In practice a full clean and seat-pocket dress is 10-15 min on a narrowbody and 25-45 on a widebody.
- The narrowbody critical path is passengers off, galley service, passengers on. With no catering exchange and two doors the Airbus minimum for an A320 is 22 min, which is the physical basis of the 25-minute LCC turn.
- The widebody critical path is usually the forward hold (787, 777) or catering (A330, A350, 747-8, A380), not passengers. That is why a domestic widebody with light catering and few containers can turn in about an hour while an international one cannot.
- Fuel is rarely critical for short sectors (15 min for 20,000-26,000 L). It becomes critical on the 777-300ER (42 min, or 56 without the modification) and on en-route stops.
- The A380 swing from 89 to 139 min shows how much the stand matters: without an upper-deck bridge and upper-deck catering trucks, 34 upper-deck trolleys go through main-deck lifts at 2 min each.

### 3.2 Scheduled versus actual

- EUROCONTROL / CODA [PUB]: mean scheduled against actual turnaround, medium category, 2018-2022: 49/57, 50/57, 50/56, 51/57, 50/59 min. Heavy: 117/131, 117/131, 116/127, 116/125, 119/134. Actual is in-block to off-block of the same airframe.
- Assaia 2026 turnaround report [PUB, trade press]: turns in North America and Europe ran 11.1 min over schedule on average in Q1 2026, across more than 2 million turnarounds; the overrun uses about 6% of stand capacity.
- CODA 2023 [PUB]: average all-causes delay 17.5 min per flight; reactionary delay 8.1 min per flight, 46% of delay minutes; airline causes 4.5 min; arrival punctuality 70.6%; 28% of flights exceeded scheduled block time; flights recovered 4.2 min between departure and arrival on average.

### 3.3 Minimum ground time against scheduled turn

Schedule-planning practice [REC, not sourced this session]:

- A minimum ground time (MGT) matrix by subfleet, station class (hub, spoke, international outstation) and domestic or international arrival and departure is a hard constraint in fleet assignment and tail routing. Scheduled turn = MGT + buffer.
- At spokes the scheduled turn sits at or just above MGT. At a banked hub it is set by the gap between the inbound and outbound bank, typically 45-75 min for a narrowbody whatever the MGT.
- Long-haul outstation turns are set by the departure-time window, curfews and crew rest more than by servicing: 90-120 min is the working minimum, 2-4 hours is common, and many aircraft sit all day.
- Crew: on a domestic network a crew staying with the aircraft needs no extra time; a crew changing aircraft needs roughly 30-60 min scheduled connection, more for international. LCC crews normally stay with one aircraft for the whole duty.
- A-CDM uses the same idea in operations: the minimum turn-round time (MTTT) for the aircraft type, airline and stand is added to the estimated in-block time to give the first target off-block time (TOBT), which the handler then updates. The milestone chain runs from flight plan activation through landing, in-block, start of ground handling, TOBT update, TSAT issue, boarding start, aircraft ready, start-up request and approval, off-block, take-off.

### 3.4 Utilisation

US carriers, average daily block hours per aircraft, Form 41 via the MIT Airline Data Project [PUB]:

| Carrier | 2015 | 2017 | 2019 | Departures per aircraft per day, 2019 | Stage length 2019 (miles) |
|---|---|---|---|---|---|
| Spirit | 13.14 | 11.50 | 12.24 | 4.58 | 1,009 |
| Frontier | 12.39 | 13.41 | 11.56 | 4.11 | 1,050 |
| JetBlue | 11.89 | 11.74 | 11.85 | 3.99 | 1,137 |
| Southwest | 10.53 | 10.68 | 10.51 | 5.10 | 748 |
| Alaska | 11.80 | 11.25 | 10.88 | 3.30 | 1,299 |
| American | 9.81 | 9.92 | 11.14 | 3.61 | 1,202 |
| Delta | 10.32 | 10.15 | 10.42 | 3.56 | 1,152 |
| United | 10.39 | 10.44 | 10.82 | 2.84 | 1,597 |
| Allegiant | 5.76 | 6.47 | 7.75 | 3.40 | 871 |
| Widebody fleets (American, Delta, United, Hawaiian) | 11.70 | 12.00 | 13.02 | — | — |

Reading: ULCC 11.5-13.4, hybrid about 11-12, network total fleet about 10-11, network widebody 12-13.7 (Delta about 13.0 throughout, American 13.7 in 2019). Allegiant is the deliberate low-utilisation exception. European figures were not retrieved; recalled values are Wizz about 12, easyJet about 11, Ryanair about 9-10 with short sectors, and Gulf long-haul fleets 13-15 [REC].

How the turn drives it: a 16-hour operating day with six sectors gives 13.9 block hours at 25-min turns, 13.1 at 35, 12.2 at 45 and 11.0 at 60. Each 5 minutes of turn on a six-sector aircraft costs about 0.4 block hours a day, or 3%. Southwest's 1978 figure makes the same point: two extra aircraft on a fleet of 13-19 for 10 more minutes per turn.

### 3.5 Delay propagation

- Europe [PUB, University of Westminster for EUROCONTROL]: each minute of primary delay generates about 0.8 min of reactionary delay network-wide (multiplier 1.8). 86-88% of reactionary delay is rotational, meaning the same airframe's later legs. The multiplier grows with the size of the primary delay: 1.49 at 5 min, 1.67 at 15, 1.94 at 30, 2.47 at 60, 3.01 at 90, 3.54 at 120, 4.61 at 180. Narrowbodies fly about five rotations a day; a 2-hour primary delay on the first causes about 4 more hours down the line.
- IATA reactionary delay codes [PUB, CODA]: 91 load connection, 93 aircraft rotation, 94 cabin crew rotation, 95 crew rotation, 96 operations control.
- United States [PUB, BTS]: in August 2026, 71.53% of flights arrived on time; 10.16% of all flights were delayed by a late-arriving aircraft, which is 39% of delayed flights; carrier-caused 7.29%, national aviation system 7.45%, weather 1.06%, security 0.05%, cancelled 2.00%, diverted 0.46%. The share of delay minutes was not on the page; 35-40% is the usual figure [REC].
- Cost [PUB, EUROCONTROL, 2022 prices]: tactical at-gate delay with network effect is EUR 166 per minute averaged over all delays and EUR 45 per minute for delays under 30 min; strategic (buffer built into the schedule) at-gate is EUR 17.78 per minute. An A320 at the gate costs EUR 95 for 5 min, 594 for 15, 2,162 for 30. US block-minute cost was USD 100.76 in 2024 [PUB, secondary citing Airlines for America].
- The buffer trade-off follows from those two numbers: a minute of turn buffer costs the strategic rate every day; a minute of tactical delay costs roughly three to ten times as much when it happens.

### 3.6 Not retrieved

Schmidt (2017), "A review of aircraft turnaround operations and simulations", Progress in Aerospace Sciences (doi:10.1016/j.paerosci.2017.05.002) is paywalled and was not read. Fricke and Schultz (ATM Seminar 2009), the Boeing Aero article on boarding rates (commonly quoted as a fall from about 20 to about 9 passengers per minute between the 1970s and 1998, consistent with the 757 chart's 9 per minute), IATA AHM turnaround material, the ATR, Dash 8, CRJ and A220 planning documents, and airline annual-report utilisation for European carriers were also not retrieved.

---

## 4. IATA SSIM Chapter 8: the MCT record

Source: SSIM Chapter 8, issue of March 2021, as published on iata.org, plus the IATA MCT User Guide v1.1 and Technical Guide v2.2 and OAG's explainers. All [PUB].

### 4.1 Definitions and governance

- MCT: the shortest interval in which a passenger and their bags can be moved from one flight to a connecting flight at a given place. Governed by IATA Passenger Standards Conference Resolution 765 and to be observed by all ticketing and reservations outlets. The chapter does not stop an agent building point-to-point segments by hand.
- Three kinds of record, all called "MCTs": station standard, exception, suppression.
- Station standard: a record with no arrival carrier, arrival operating carrier, departure carrier or departure operating carrier. Agreed by the airlines operating at the airport (OAG calls this the Airport Operators Committee), submitted to IATA for approval, and published by IATA to the data aggregators.
- Exception: a bilateral agreement or a carrier's own online interval. May be lower or higher than the station standard.
- Suppression: blocks a connection for a status pair, at a station or across a state, country, region or the world.
- Global defaults when no station standard exists: DD 0030, DI 0100, ID 0130, II 0130; inter-station 0400 for all status pairs. (OAG's explainer shows the same values. The many 20 / 60 / 60 / 60 rows in the Ethiopian station table suggest that was the earlier default; that is an inference.)
- Maintained by the Minimum Connecting Times Group under the Plan Standards Board.

### 4.2 Filing and distribution

1. A carrier prepares the exception in the IATA template and emails it to both aggregators (OAG and Cirium, formerly Innovata).
2. The receiving carrier must concur, except when the change is administrative (flight-number reassignment), when the filer is itself the receiving carrier, or when the record is a suppression. Removing a suppression against a named carrier does need concurrence.
3. The aggregators check for duplicates and overlaps, load, and distribute to GDSs and other systems. OAG says its file is refreshed daily; its FAQ also refers to GDSs that take the data only weekly. Cirium's frequency was not found. Expired records are kept for a year for reference and are not sent to GDSs.
4. A change is a delete (action D, matching the stored record exactly) plus an add (action A). Within a station, deletes are processed before adds; if any record for a station is rejected, nothing for that station is processed.

Scale [PUB, OAG]: more than 157,000 standards and exceptions; London Heathrow has 2,372 carrier exceptions and Paris CDG about 10,160.

### 4.3 What changed in the 2018 standard

The data-element set was added in the 2018 SSIM; the user guide dates from November 2018, and its examples end-date old-style filings on 31 October 2019, which suggests the industry cut-over was late 2019 (inference). Compared with the old format:

- Time is HHMM, not minutes.
- New: arrival and departure codeshare indicator and codeshare operating carrier; suppression indicator with suppression region, country and state; submitting carrier; filing date; action indicator.
- Dropped: previous and next city (station, state, country and region remain).
- Aircraft type and body became separate fields that cannot be combined.
- Suppression is an explicit Y flag instead of a 999-minute time.
- From 1 November 2022: the connection-building filter, a per-carrier list of permitted interline partners (record type 3), with the trailer becoming record type 4.

### 4.4 Record layout

Fixed 200-byte records, ASCII or EBCDIC. Record type 1 is the header, 2 the MCT record, 3 the partner list (since November 2022), 4 the trailer (type 3 before then).

Header (type 1): bytes 2-31 "MINIMUM CONNECT TIME DATA SET"; 32-66 creator; 67-73 creation date DDMMMYY (UTC); 74-77 creation time; 78 content indicator, F for full replacement or U for updates; 195-200 serial 000001.

MCT record (type 2):

| Bytes | Element | Notes |
|---|---|---|
| 1 | Record type | 2 |
| 2-4 | Arrival station | Blank only for a global suppression |
| 5-8 | Time | HHMM; blank for a suppression |
| 9-10 | International/domestic status | DD, DI, ID, II. Mandatory on every record |
| 11-13 | Departure station | Always populated, even when equal to arrival; differs for inter-station |
| 14-15 | Arrival carrier | The marketing designator on the ticket |
| 16 | Arrival codeshare indicator | Y or blank |
| 17-18 | Arrival codeshare operating carrier | Only with indicator Y |
| 19-20 | Departure carrier | |
| 21 | Departure codeshare indicator | Y or blank |
| 22-23 | Departure codeshare operating carrier | |
| 24-26 | Arrival aircraft type | IATA 3-character type; exact match to the schedule; not with body |
| 27 | Arrival aircraft body | W or N; not with type |
| 28-30 | Departure aircraft type | Surface types such as BUS and TRN go here |
| 31 | Departure aircraft body | |
| 32-33 | Arrival terminal | Left-justified |
| 34-35 | Departure terminal | |
| 36-37 | Previous country | ISO; required with previous state |
| 38-40 | Previous station | Excludes country, state and region |
| 41-42 | Next country | |
| 43-45 | Next station | |
| 46-49, 50-53 | Arrival flight number range start, end | Four digits; equal for a single flight |
| 54-57, 58-61 | Departure flight number range start, end | |
| 62-63 | Previous state | |
| 64-65 | Next state | |
| 66-68 | Previous region | Excludes other previous-location fields |
| 69-71 | Next region | |
| 72-78 | Effective from (local) | DDMMMYY; applies to the departing flight from 00:00 |
| 79-85 | Effective to (local) | Through 23:59 |
| 87 | Suppression indicator | Y or N; blank on a station standard |
| 88-90 | Suppression region | |
| 91-92 | Suppression country | |
| 93-94 | Suppression state | Requires country |
| 95-96 | Submitting carrier | Set by the aggregator; blank on a station standard |
| 97-103 | Filing date | Informational |
| 104 | Action indicator | A or D; blank in a full file |
| 195-200 | Record serial number | |

Partner list (type 3): bytes 2-4 submitting airline; 5-194 up to 95 two-character permitted partners; several records per carrier allowed. Trailer: byte 194 "E"; 195-200 serial check.

Sample line from the chapter: `2CDG0130IICDGDLYAFDLY9W        2C2E` is CDG, 1h30, II, arriving on a Delta-marketed Air France-operated flight, departing on a Delta-marketed Jet Airways-operated flight, terminal 2C to 2E.

### 4.5 Hierarchy

The priority table, most significant first. A record is compared with another by finding the first element in this order that is populated in exactly one of them; that record wins.

| # | Element | # | Element |
|---|---|---|---|
| 1 | Departure codeshare indicator | 17 | Next country |
| 2 | Departure carrier | 18 | Previous country |
| 3 | Departure codeshare operating carrier | 19 | Next region |
| 4 | Arrival codeshare indicator | 20 | Previous region |
| 5 | Arrival carrier | 21 | Departure aircraft type |
| 6 | Arrival codeshare operating carrier | 22 | Arrival aircraft type |
| 7, 8 | Departure flight number range start, end | 23 | Departure aircraft body |
| 9, 10 | Arrival flight number range start, end | 24 | Arrival aircraft body |
| 11 | Departure terminal | 25 | Effective from date |
| 12 | Arrival terminal | 26 | Effective to date |
| 13 | Next station | 27 | Departure station |
| 14 | Previous station | 28 | Arrival station |
| 15 | Next state | 29 | Suppression state |
| 16 | Previous state | 30, 31 | Suppression country, region |

Rules:

- Any station standard overrides the global default. Any exception or suppression (station-specific or geographic) overrides any station standard.
- Departure side outranks arrival side throughout, so a receiving carrier's record beats a delivering carrier's.
- Ties among records with the same populated fields: narrower departure flight range, then narrower arrival flight range, then next region, previous region and suppression region with SCH (Schengen) overriding EUR. Anything still tied is a duplicate and should have been rejected by the aggregator.
- A dated record beats an undated one. Date ranges that overlap on otherwise identical records are duplicates.
- An exception outranks a global suppression, so a carrier can suppress all connections with a partner worldwide and then file exceptions at the stations where it wants them.
- Record order in the file means nothing.

Codeshares:

- Codeshare is detected by DEI 50 on the schedule leg. A record without Y applies to operating flights only.
- A marketing flight uses the operating carrier's MCT unless the marketing carrier files its own Y record, which then overrides. The chapter says a marketing record is only needed when the marketing carrier wants a longer time.
- Indicator Y with no operating carrier: all of that marketing carrier's codeshares. Indicator Y with operating carrier and blank marketing carrier: every codeshare operated by that carrier.
- With both a marketing and an operating carrier on a record, a flight number range applies to the marketing carrier.
- Example from the chapter: LHR to MIA on BA 207, also sold as AA 6160. Records: MIA ID 0140 for AA-marketed BA-operated to AA, and MIA ID 0130 for BA to AA. BA 207 connects to both the 16:35 and 17:00 departures; AA 6160 connects only to the 17:00.

Status and terminals come from the schedule: the leg's MCT international/domestic status and passenger terminal fields in the chapter 7 file (believed to be bytes 120-121 and 53-54 / 71-72 of the type 3 record; check against your copy), with DEI 220 overriding the status where leg and segment interpretations differ. Aircraft type must match the schedule exactly: an MCT filed for 737 does not match a leg filed as 73H.

Connection-building filter: if both carriers file partner lists and either omits the other, no connection is built, whatever MCT records exist. A carrier that files no list accepts everyone. Online connections are always allowed. Both the marketing and the operating carrier are tested. It applies per connection, not per itinerary.

### 4.6 GDS entries

- Station display, one row from Travelport's cross-GDS comparison [PUB]: Travelport+ and Apollo `DCTJFK`, Worldspan `KEC*JFK`, Amadeus `DMJFK`, Sabre `T*CT-JFK`.
- Travelport [PUB, formats guide and Smartpoint help]: `DCTPDX-DD` or `DCTNCE-II` limits to a status pair; `DCTNCE` includes exceptions; `DCTAMS*BA`, `DCTNCE*AFAF` and `DCTAMS*DLKL-II` filter by carrier or carrier pair; `DCT` and `DCTV` open display and validation masks; `@MT` or `@MT1*3` (Apollo `³MT`) checks booked segments. The connection-building filter is visible too: `DCT*WN`, `DCT*QFSQ`. Connections produced by availability and fare shopping already meet MCT; hand-built ones are checked at end transact, which returns a warning and a link to the table. The help page's sample is a booking failing a 40-minute DD minimum at Salt Lake City.
- Amadeus, from unofficial quick-reference guides [secondary]: `DMFRA`, `DMFRA/15DEC` (dated), `DMLON` (multi-airport city), `DM BA/LHR/AA` (carrier pair), `DM BA/LHR-LGW/AA` (inter-airport). `DMI` for the booked itinerary is [REC].
- Sabre: `VCT*` to verify booked segments is [REC]. Sabre's booking APIs validate MCT between new or existing segments under a `haltOnInvalidMCT` control, and a host message beginning "CONNECTION SUPPRESSED" means the pair must be verified with the carrier [PUB, search extract of Sabre's developer page].
- Not found for any GDS: the exact warning strings, and default maximum connecting times.
- ExpertFlyer and KVS expose the same table to the public [PUB, secondary].

---

## 5. Commercial findings around MCT

### 5.1 Below MCT

- Availability and shopping engines build connections only at or above the applicable MCT, so a sub-MCT pair never appears as a through itinerary and cannot be priced with a through fare [PUB, Travelport; SSIM].
- An agent can still sell the two flights as separate segments. The GDS warns at end transact. Airlines generally will not waive MCT on one ticket [PUB, secondary].
- Shorter MCT improves display position because elapsed time ranks connections, which is why carriers file aggressive online exceptions and why OAG calls MCT "a powerful marketing tool" [PUB, OAG].
- Maximum connecting time is not standardised; each GDS sets its own [PUB, OAG FAQ].

### 5.2 Protected and self-connect

- One ticket at or above MCT: bags through-checked, the carrier re-accommodates after a misconnect, and in the EU compensation rules apply to delay at the final destination.
- Separate tickets: no protection unless a product supplies it. Ryanair has sold connecting itineraries since April 2017, first at Rome Fiumicino, with rebooking at no cost after a misconnect [PUB, Wikipedia]. Airport and third-party self-connect products (Gatwick, Milan, Kiwi-style guarantees) use minimums of roughly 2-3 hours [REC]. A consumer guide recommends 3-4 hours for unprotected self-connections [PUB, secondary].

### 5.3 Misconnection rates

No published curve of misconnect share against connection time was found. What was retrieved:

- United States, 2007 [PUB, NEXTOR Total Delay Impact Study]: 3.4% of 487 million boarded passengers were disrupted (16.4 million); 30.3% of those by a missed connection and 69.7% by a cancellation. That is about 1.0% of all boarded passengers. If roughly a third of passengers were connecting, about 3% of connecting passengers misconnected (derived, and 2007 was a poor year). Disrupted passengers averaged 457 min of delay against 16 min for everyone else. Missed connections accounted for USD 1.5 billion of USD 16.7 billion passenger delay cost.
- The 2-5% hub range in the brief is consistent with that but was not sourced.
- A modelled curve is given in section 6.4.

### 5.4 Baggage

SITA Baggage IT Insights [PUB for 2025 figures; earlier years REC]:

- Mishandled bags per 1,000 passengers: 18.9 (2007), 5.6 (2019), 3.5 (2020), 4.35 (2021), 7.6 (2022), 6.9 (2023), 6.3 (2024) [REC]. The 2026 edition reports mishandling down 20% in 2025 on 4% more passengers [PUB], which implies about 5.
- Transfer mishandling share: 42% (2022), 46% (2023), 41% (2024) [REC]; 39% (2025) [PUB]. Always the largest cause.
- Cost per mishandled bag, restated by SITA at USD 260 on average: delayed 245 (170 operations, 75 compensation), damaged 255, lost 635. Industry cost USD 6.3 billion in 2025 [PUB].

### 5.5 Cost of a misconnected passenger

- Airline hard cost (rebooking, care, compensation), base scenario, 2010 euros per passenger per minute of delay [PUB, University of Westminster]: 0.14 at 15 min, 0.24 at 30, 0.41 at 60, 0.56 at 90, 0.70 at 120, 0.96 at 180, 1.20 at 240, 1.44 at 300. A misconnect that delivers the passenger 3 hours late costs about EUR 170; 5 hours, about EUR 430. Soft cost (lost future revenue) averages EUR 0.18 per passenger-minute.
- Add a delayed bag at USD 245 in a share of cases, an overnight stay where the next flight is the following day, and the displaced seat on the rebooked flight when it would otherwise have sold. A working figure of USD 150-250 for a same-day misconnect and USD 400-700 for an overnight one is an ESTIMATE built from those components.

### 5.6 Banks, depeaking and re-banking

- Bank structure: arrivals in a wave, then departures, so that connection times cluster just above MCT. Costs are peaked staffing and gates, congestion, and aircraft idling at spokes [PUB, Wikipedia].
- Doha, Qatar Airways, March 2020 [PUB, Wikipedia]: three waves, 05:00-09:00 (132 movements), 16:00-21:00 (128) and 23:00-03:00 (132), 471 daily movements.
- Rolling hubs: American was first to depeak after September 2001 and re-banked in 2015, judging that connecting revenue outweighed the cost [PUB, Wikipedia]. Detail from trade press [REC]: Chicago in April 2002 and Dallas/Fort Worth in November 2002, with savings put at about USD 100 million a year; Delta depeaked Atlanta in January 2005; American re-banked Miami in August 2014 and Chicago and Dallas/Fort Worth in March 2015 into roughly 9-13 banks a day, with a revenue gain guided in the low hundreds of millions of dollars a year. None of these figures was re-verified.
- Bank counts for other hubs [REC]: Frankfurt and Munich 4-5 waves, Amsterdam about 7, Paris CDG about 6, Dubai 3. Atlanta and the American hubs between 2002 and 2015 ran continuously.
- The link to MCT: a bank is only as tight as the MCT allows. A hub with a 30-45 minute MCT can run a bank with a 60-90 minute cycle; a hub at 60-90 needs two hours or more, which costs aircraft time and limits the number of banks in a day.

---

## 6. Model recommendations

### 6.1 Turnaround formula

Compute three paths and take the longest. All times in minutes.

```
pax_off  = seats_in  * load_factor_in  * exchange   (exchange = 1.0 for a turn, 0.5-0.8 for a through stop)
pax_on   = seats_out * load_factor_out * exchange

Passenger path
  t_on       = 2 (bridge) or 1 (stairs)
  t_deplane  = pax_off / (doors * r_off)
  t_cabin    = cabin window: see table below
  t_board    = pax_on / (doors * r_on) + lps
  t_off      = 1.5
  P = t_on + t_deplane + t_cabin + t_board + t_off

Hold path
  bulk:        C = 2 + bags/(15*loaders) + bags/(10*loaders) + 1.5      bags = 1.2 per passenger
  containers:  C = 2.5 + n_fwd * (1.2 + 1.4) + 2.5                      forward hold governs

Fuel path
  F = 3 + uplift_litres / flow + 3

minimum_turn = max(P, C, F) + pushback_allowance (0-2)
scheduled_turn = minimum_turn + buffer
```

Parameters:

| Parameter | ULCC / LCC | Network short-haul | Long-haul |
|---|---|---|---|
| doors | 2 at stairs or dual-bridge stands, else 1 | 1 | 2 (1 for 767 and 787 in the Boeing charts) |
| r_off per door, narrowbody | 17 | 18-20 | — |
| r_on per door, narrowbody | 10 | 9-12 (9 with heavy carry-on) | — |
| r_off / r_on per door, widebody | 25 / 15 | 25 / 15 | 25 / 15 (Boeing single door: 40 / 25) |
| lps | 2 | 2 | 4 |
| t_cabin | 4-5: crew tidy, no galley exchange | 10-14: cleaners plus 6-11 trolleys | 30-45: full clean and dress, 40-85 trolleys, security search |
| uplift | sector fuel, often tankered so zero on alternate legs | sector fuel | sector fuel; 60,000-170,000 L |
| flow, L/min | 1,250 | 1,250 (2,000 for 757) | 3,500-4,300; 5,400 for 747 |
| buffer | 0-5 | 5-15 at spokes; bank-driven at hubs | 20-60 |

Checks against the evidence: A320 with 180 seats at 90% load, two stairs, LCC parameters gives 1 + 4.8 + 5 + 8.1 + 2 + 1.5 = 22.4, so 25 scheduled (Airbus's own minimum chart says 22). A 737-800 with 162 passengers, one door, network parameters gives 2 + 9 + 12 + 13.5 + 2 + 1.5 = 40 against Boeing's 37.5. A 787-9 is governed by the hold: 2.5 + 16 x 2.6 + 2.5 = 46.6 against Boeing's 51.

Simpler defaults if the simulator has only seats and carrier model:

```
LCC narrowbody:        min = 12 + 0.07 * seats             (180 seats: 25; 239 seats: 29)
Network narrowbody:    min = 15 + 0.15 * seats             (162 seats: 39; 190 seats: 44)
Regional (under 100):  min = 14 + 0.12 * seats             (76 seats: 23)
Widebody, domestic:    min = 35 + 0.07 * seats             (300 seats: 56)
Widebody, long-haul:   min = 60 + 0.10 * seats             (300 seats: 90; 500 seats: 110)
scheduled = min + 5 (LCC), + 10 (network spoke), + 30 (long-haul outstation); at a banked hub, the bank gap
```

Actual turn time: draw an overrun with mean about 8-11 min and a long right tail on top of the scheduled turn, or more simply let the departure be late by max(0, inbound delay minus buffer) plus a small independent handling delay. Apply the reactionary multipliers in 3.5 as a sanity check on the aggregate: reactionary delay should come out at 40-50% of delay minutes.

### 6.2 Default MCTs by airport class

When a station standard exists, use it. When nothing is filed, a reservation system must use the IATA global defaults (30 / 60 / 90 / 90, inter-station 240). The table below is for the simulator, to generate plausible station standards for airports with no published value.

| Airport class | DD | DI | ID | II | Examples |
|---|---|---|---|---|---|
| Small single-terminal airport | 20-30 | 45 | 60 | 45-60 | regional airports |
| Medium single-terminal | 30-40 | 45-60 | 60-75 | 45-60 | BRU, DUB, NBO |
| Efficient single-roof hub | 30-40 | 30-45 | 40-60 | 30-45 | MUC T2, VIE, ZRH, HEL, FRA, AMS, DOH |
| Large hub, terminals linked airside | 45-55 | 60 | 75-90 | 60-75 | ATL, DFW, DEN, SIN, HKG, IST |
| Mega hub, landside or bus between terminals | 60-70 same terminal; 90-120 across | 75-90 | 105-120 | 90-120 | LAX, JFK, LHR, CDG, MEX |
| Legacy split domestic and international terminals | 30-60 | 120-180 | 150-180 | 90 | older DEL, BOM, LOS, JED |
| Inter-airport | 180-240 | 180-240 | 180-240 | 180-240 | LHR-LGW, NRT-HND |

Adjustments: add 30 to ID and II anywhere an international arrival must reclaim and recheck bags (United States and Canada unless precleared; treat precleared arrivals as domestic). Subtract 10-20 for the hub carrier's online same-terminal connections. Add 15-30 for a terminal change with an airside link and 45-60 for a landside one. Treat intra-Schengen flights as domestic-like for the purpose of time, and add 10-15 where a connection crosses the Schengen border.

### 6.3 The exception hierarchy to implement

1. Load record types 2 and 3. Keep station standards (all four carrier fields blank), exceptions and suppressions in one table keyed by status and station pair, with global suppressions under a wildcard key.
2. For a candidate connection, derive from the schedule: status pair, arrival and departure station and terminal, marketing carrier and flight number on each side, operating carrier from DEI 50, aircraft type and body, previous and next station with their state, country and region, and the local departure date at the connect point.
3. Apply the partner filter first. If either marketing or operating carrier on one side has filed a list that omits a carrier on the other side, stop: no connection. Online connections always pass.
4. Collect matching records. A record matches when every populated field agrees with the connection and the departure date is inside the effective range. For carrier fields, run the lookup for the marketing flight (records with indicator Y) and, if none matches, again as the operating flight (records without Y, using the DEI 50 carrier and flight).
5. Rank: exceptions and suppressions above station standards above global defaults. Within a class, compare on the 31-element priority order; first element populated in exactly one record wins. Then the tie-breaks in 4.5.
6. If the winner is a suppression, do not build. Otherwise build only if scheduled departure minus scheduled arrival is at least the record's time.
7. Store which record decided, so a rejected sale can be explained ("MCT ID 01:30 at MIA, BA to AA, filed by AA").
8. On sell of hand-built segments, run the same check and return a warning with the applicable time, not a hard reject, to match GDS behaviour. Refuse to price the pair as a through fare.

Implementation notes: index by (arrival station, departure station, status) and keep the candidate list small; precompute for each record a 31-bit mask of populated fields so the comparison in step 5 is an integer comparison after masking out fields populated in both; accept update files (content indicator U) by applying deletes before adds per station and rejecting the whole station on any error.

### 6.4 Misconnect probability

No published curve was found, so this is a model, calibrated to a typical arrival-delay distribution and checked against the NEXTOR figure of roughly 3% of connecting passengers.

Given the simulator's own delays, the passenger makes the connection if the time actually available is at least the time needed:

```
slack      = scheduled_connection - MCT
available  = scheduled_connection - inbound_arrival_delay + outbound_departure_delay
needed     = MCT - margin + noise          margin about 10 min (MCT is conservative for most passengers)
                                           noise sd about 5 min (seat row, gate distance, queue)

P(miss) = logistic( (inbound_delay - outbound_delay - slack - 10) / 5 )
```

Values of P(miss) with an on-time outbound flight:

| Slack over MCT | Inbound 0 late | 10 | 20 | 30 | 45 | 60 |
|---|---|---|---|---|---|---|
| 0 | 0.12 | 0.50 | 0.88 | 0.98 | 1.00 | 1.00 |
| 15 | 0.01 | 0.05 | 0.27 | 0.73 | 0.98 | 1.00 |
| 30 | 0.00 | 0.00 | 0.02 | 0.12 | 0.73 | 0.98 |
| 60 | 0.00 | 0.00 | 0.00 | 0.00 | 0.01 | 0.12 |

Early arrivals are negative delay and help. A hub that holds departures for connecting passengers is an outbound delay chosen by the airline; model it as a hold of up to 5-10 min when the number of inbound connectors at risk exceeds a threshold.

If the simulator needs an unconditional curve (for scoring itineraries at booking time), integrate over the inbound delay distribution. With P(arrival delay > x) = 0.17 exp(-(x - 15)/30) + 0.03 exp(-(x - 15)/110) for x of 15 or more (20% of flights more than 15 min late, 13% more than 30, 6% more than 60, 1.7% more than 120), and about 30% of at-risk connections saved by a late or held outbound:

| Slack over MCT (min) | 0 | 15 | 30 | 45 | 60 | 90 | 120 | 180 |
|---|---|---|---|---|---|---|---|---|
| P(miss) | 0.18 | 0.10 | 0.07 | 0.05 | 0.03 | 0.02 | 0.01 | 0.005 |

At a hub with a 40-minute MCT that reads: 45-minute connection about 14%, 60 about 9%, 90 about 4%, 120 about 2%. Averaged over a bank structure with mean slack of 40-50 min it gives 4-5%, at the top of the 2-5% range.

Bags: use the same form with the margin reversed, because a bag needs more time than its owner at short connections: P(bag miss) = logistic((inbound_delay - outbound_delay - slack + 5) / 8), and never hold a flight for bags. Calibrate so that transfer bags produce about 40% of a total of 5-7 mishandled bags per 1,000 passengers.

Consequences to simulate after a miss: rebook on the next flight with a seat (delay = wait for it; the NEXTOR average for disrupted passengers was 7.6 hours), charge the hard cost from 5.5, and add an overnight cost when no same-day option exists.

---

## 7. Sources

Manufacturer documents
- Boeing, Airplane Characteristics for Airport Planning: 737 NG (D6-58325-7, Rev C), 737 MAX (Rev K), 757 (Rev H), 767 (Rev K), 777-200LR/-300ER/F (Rev G), 787 (Rev Q), 747-8 (Rev E). https://www.boeing.com/commercial/airports/plan-manuals
- Airbus, Aircraft Characteristics - Airport and Maintenance Planning: A318 (2012), A319, A320, A321 (May 2017), A330, A350-900 (preliminary, 2012), A380, via the SKYbrary bookshelf (https://skybrary.aero/sites/default/files/bookshelf/2367.pdf to 2375.pdf); A321 March 2022 revision at https://aviation.hawaii.gov/wp-content/uploads/2025/06/Airbus-techdata-AC_A321_0322-2.pdf
- Embraer Airport Planning Manuals: E190 (APM 1901, Rev 24, November 2024) https://embraer.com/media/jwsj1u1u/e190apm_apm_190-1.pdf ; E-Jets E2 (APM 5824, Rev 20) https://www.embraer.com/media/vmsjxvk5/e-jets-e2_apm_e-jetse2.pdf

Turnaround, delay and utilisation
- EUROCONTROL Standard Inputs for Economic Analyses, chapters 16 (cost of delay) and 18 (turnaround time): https://standard-inputs.netlify.app/chapters/turnaround_time.html and .../cost_of_delay.html
- EUROCONTROL CODA Digest, All-Causes Delays to Air Transport in Europe, Annual 2023: https://www.eurocontrol.int/sites/default/files/2024-12/eurocontrol-coda-digest-annual-report-2023.pdf
- University of Westminster for EUROCONTROL, European airline delay cost reference values, v4.1 (2015): https://www.eurocontrol.int/sites/default/files/publication/files/european-airline-delay-cost-reference-values-final-report-4-1.pdf
- NEXTOR, Total Delay Impact Study (2010): https://isr.umd.edu/NEXTOR/pubs/TDI_Report_Final_10_18_10_V3.pdf
- US BTS, airline on-time statistics and delay causes: https://www.transtats.bts.gov/OT_Delay/OT_DelayCause1.asp
- MIT Airline Data Project (Form 41): https://web.mit.edu/airlinedata/www/Aircraft&Related.html
- HAW Hamburg master's thesis, Analysis of Ground Handling Characteristics of Innovative Aircraft Configurations (observed turnarounds, appendix A): https://www.fzt.haw-hamburg.de/pers/Scholz/arbeiten/TextRico.pdf
- Schultz, Faster aircraft boarding enabled by infrastructural changes, Winter Simulation Conference 2017: https://www.informs-sim.org/wsc17papers/includes/files/208.pdf
- Assaia 2026 Turnaround Report, as reported by AviationPros: https://www.aviationpros.com/ground-support-worldwide/ground-handling/news/55411432/study-finds-aircraft-turnarounds-average-11-minutes-longer-than-planned
- Secondary: https://www.airtraveler.club/insider/airport-operations/6-airline-turnaround-times-targets-versus-what-actually-gets-measured/ ; https://aerocorner.com/blog/airline-aircraft-turnaround/ ; https://thepointsguy.com/news/what-it-takes-to-get-a-plane-ready-between-flights/ ; https://en.wikipedia.org/wiki/History_of_Southwest_Airlines ; https://en.wikipedia.org/wiki/Ryanair

MCT
- IATA SSIM Chapter 8 (March 2021): https://www.iata.org/contentassets/638f0938b3dd451b872a1d8357755421/ssim2021_chapter-8-mctstandards.pdf
- IATA MCT User Guide v1.1: https://www.iata.org/contentassets/638f0938b3dd451b872a1d8357755421/minimum-connecting-time-user-guide_version-1.1.pdf
- IATA MCT Technical Guide v2.2: https://www.iata.org/contentassets/638f0938b3dd451b872a1d8357755421/minimum-connecting-time-technical-guide_version-2.2.pdf
- OAG: https://www.oag.com/minimum-connection-time ; https://www.oag.com/hubfs/Inbound-Services/OAG-Guide-to-MCTs-Explained.pdf ; https://www.oag.com/hubfs/Inbound-Services/OAG%20MCT+Carrier+FAQs.pdf
- Travelport help, Minimum Connect Time: https://support.travelport.com/webhelp/Smartpoint1G1V/Content/Air/RelatedTopics/MinimumConnectTime.htm ; formats guide https://support.travelport.com/webhelp/Formats/Content/TravelInfo/MinimumConnectTime.htm ; cross-GDS comparison https://support.travelport.com/webhelp/Formats/Content/FormatCompare/Miscellaneous.htm
- Sabre developer page on MCT validation (seen as search extract only): https://developer.sabre.com/minimum-connecting-time-validation
- ANA transit page (search extract only): https://www.ana.co.jp/en/us/guide/boarding-procedures/checkin/domestic/transit/
- Asiana airport guide (search extract only): https://flyasiana.com/C/US/EN/travel/airport
- JAL transit pages (search extract only): https://www.jal.co.jp/jp/en/inter/airport/doh/transit/ ; https://www.jal.co.jp/en/inter/airport/sin/transit/index.html
- Qatar Airways trade notice on Doha MCT: https://content.app-us1.com/10Y6m/2026/03/19/8e779eda-dfd4-4d2b-a900-21fa1f45c8b7.pdf
- Hamad International Airport transfers: https://dohahamadairport.com/airport-guide/at-the-airport/transfers
- Haneda transit pages: https://tokyo-haneda.com/en/flight/transit/int_dms.html ; https://tokyo-haneda.com/en/flight/transit/dms_int.html
- Changi transit guide: https://www.changiairport.com/en/fly/transit-guide.html
- Qantas MCT change, trade press: https://travelweekly.com.au/qantas-increases-international-connection-time-reduce-mishandled-baggage/
- flydubai Terminal 3 operations (search extract only): https://www.flydubai.com/en/flying-with-us/terminal-3-operations
- Ethiopian Airlines, minimum connecting time by station: https://www.ethiopianairlines.com/jp/en/services/services-at-the-airport/minimum-connecting-time
- Gatwick Airport, flight connections: https://www.gatwickairport.com/passenger-guides/flight-connections.html
- Heathrow, travel between airports: https://www.heathrow.com/transport-and-directions/travel-between-airports
- Secondary: https://travelsort.com/what-is-minimum-connection-time-at-airports/ ; https://airlinefyi.com/guide/airport-operations/connecting-at-airports/ ; https://en.wikipedia.org/wiki/Helsinki_Airport ; https://en.wikivoyage.org/wiki/Istanbul_Airport ; https://en.wikivoyage.org/wiki/Munich_Airport ; https://en.wikipedia.org/wiki/Airline_hub
- SITA Baggage IT Insights: https://www.sita.aero/resources/surveys-reports/sita-baggage-it-insights-2025/

## 8. Gaps to close

1. Station-standard MCTs for most hubs in Table 2 are estimates. The fix is the MCT file from OAG or Cirium, or a GDS station display per airport. Airport operators' passenger pages (Heathrow, Frankfurt, Munich, Vienna, Zurich, Madrid, Istanbul, Helsinki were checked) do not publish numbers.
2. Carrier online exceptions at US hubs, and Air Canada's sector-based values at Toronto, were not retrieved.
3. Amadeus and Sabre station-display entries are confirmed; their itinerary-check entries, the warning texts and any default maximum connecting time are not. Values tagged "search extract" came from a search-engine snippet of a page that blocked direct reading, and should be re-checked before being hard-coded. Likely sources for a second pass: JAL's per-airport transit pages, the ANA transit page, the Asiana airport guide, the Qantas agency notice of August 2022, and Emirates' terminal-transfer page.
4. ATR, Dash 8, CRJ and A220 turnaround charts were not retrieved.
5. Bank counts, depeaking savings and re-banking revenue are recalled from trade press.
6. No published misconnect-versus-connection-time curve was found; 6.4 is a model.
