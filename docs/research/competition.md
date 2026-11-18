# How airlines compete on price and capacity: evidence base for a competitor-behaviour model

Prepared 18 Nov 2026. Purpose: design input for the reaction functions of several hundred AI-run carriers.

## 0. Method and confidence tags

Every factual claim carries one of three tags.

- **[V]** verified this session against the cited source (court filing, paper, regulator release, Wikipedia with its citations, or contemporaneous press read directly or through a search-result excerpt).
- **[R]** recalled from the literature or press and **not re-verified this session**. The session's shared web-search budget ran out about a third of the way through, and many news sites (FareCompare, CS Monitor, Bloomberg, Texas Monthly, FlightGlobal, Justia, ATPCO) refused direct fetches. Treat [R] numbers as leads to check before they are quoted anywhere public. They are still good enough to set parameter ranges.
- **ESTIMATE** my own inference or proposed parameter value.

Paywalled WSJ/NYT pieces are cited by title, author and date; the facts attributed to them come from the accessible sources named beside them.

The strongest primary material found (all read in full or in the relevant part):

- Borenstein, "Rapid Communication and Price Fixing: The Airline Tariff Publishing Company Case" (1997 draft of the Kwoka–White *Antitrust Revolution* chapter).
- DOJ Competitive Impact Statement, *US v. Airline Tariff Publishing Co.*, 59 Fed. Reg. (31 Mar 1994).
- DOJ complaint, *US v. AMR Corp.* (D. Kan., filed 13 May 1999) and the Tenth Circuit opinion, 335 F.3d 1109 (2003).
- *Spirit Airlines v. Northwest Airlines*, 431 F.3d 917 (6th Cir. 2005).
- DOJ complaint, *US v. American Airlines and Robert L. Crandall* (N.D. Tex., filed 23 Feb 1983), with the call transcript.
- Goolsbee & Syverson (QJE 2008, NBER w11072); Brueckner, Lee & Singer (2010 IATA briefing of the 2013 paper, with the results table); Ciliberto & Williams (working paper); Gerardi & Shapiro (Boston Fed WP 07-7); Borenstein (NBER w16744).
- Bundeskartellamt press releases, Jan–Apr 2002 (Lufthansa/Germania).
- *In re Domestic Airline Travel Antitrust Litigation*, D.D.C., memorandum opinion of 5 Sep 2023.

---

## 1. Mechanics of fare changes and matching

### 1.1 The filing cycle

- ATPCO is the airline-owned clearing house. Carriers submit new, changed and cancelled fares; ATPCO compiles them and transmits to the CRS/GDSs and to every other airline. A fare record carries a fare basis code, origin and destination, price, first/last ticket dates, first/last travel dates and restrictions; dates and restrictions sit in a "footnote" identified by a "footnote designator". [V, Borenstein 1997]
- Early 1990s: "at least once a day" ATPCO sent a file with "thousands of fare changes"; a change became visible to travel agents and all rivals "by the following morning". [V, Borenstein 1997]
- 2010s: North American fares went out **four times each weekday, at 10:00, 13:00, 16:00 and 20:00 Eastern, and once on Saturday and Sunday at 17:00**; the 16:00 feed was a later addition to the other three. International fares went out roughly hourly, except for some hours on Saturday night. [V, N. Kralev blog and a FlyerTalk post, both undated, via search excerpts]
- Still a fixed-time domestic cycle in February 2020: a JetBlue pricing analyst planned to "flash" American "in the 4 p.m. ATPCO submission" (Northeast Alliance trial record, as reported by View from the Wing). [V]
- The 1994 consent decree barred ATPCO from giving airlines fare changes before the CRSs received them, and from updating more often than the CRSs did. [V, CIS]

**Model implication.** Price moves are discrete and public. A domestic US-style market has 4 decision ticks per weekday and 1 per weekend day; an international or modern market can run hourly. Nobody moves secretly on *published* fares. What stays private is inventory (seats per bucket) and negotiated corporate discounts (section 2.3).

### 1.2 How pricing analysts watch each other

- Each airline produced a **daily internal fare-change report**. DOJ obtained them; they contained lines such as "we are waiting to see if [carrier] is going to go along with our proposed increase", "we are abandoning the increase on [city1]-[city2] because [carrier] has not matched" and "[carrier] is now on board for the [date] increase to [fare level]". [V, Borenstein 1997]
- Analysts own groups of markets and compare each transmission against their own fares. A filing that is refiled unchanged reappears on the rival's daily list of new fares, so refiling is a way to make sure a rival has noticed. [V, Borenstein 1997]
- Published fares are observable; bucket sales are not. But rivals can see whether a bucket is **open or closed**, so no airline can undercut its rivals in secret by keeping a low price open when theirs are closed. [V, Borenstein 1997]
- American's DFW playbook added physical intelligence: investigate the entrant's financial resources, compute its break-even load factor, and "conduct head counts at the departure gate to monitor their passenger loads". [V, DOJ complaint ¶28]

### 1.3 The weekend increase ritual

Borenstein, citing James S. Hirsch, "Fliers Discover They Don't Fare Well on the Weekends", WSJ, 30 Jul 1993, B1: since the December 1992 settlement airlines have made most of their fare changes at weekends, when few tickets are sold. A carrier posts an increase Friday afternoon; it is live in the CRSs Saturday morning; if rivals in those markets have not matched by Sunday afternoon the carrier withdraws it Sunday night, and Monday opens at the old prices; if they match, the higher fares stay. [V, paraphrased]

DOJ accepted two scenarios, written into a memorandum filed with the 1994 settlement at American's request, as **not** violating the decree. They are the best compact description of the legal equilibrium behaviour: [V, Borenstein 1997, quoting the memorandum]

1. "At noon on Friday Airline A transmits 10% fare increases on certain city-pairs to ATPCO. The increased fares become available for sale through CRS at 5 p.m. that same day. On Saturday, Airline B transmits 5% increases to ATPCO on the same city-pairs. Airline A withdraws its 10% fare increases on Sunday when it learns that competing airlines have not offered matching fares for sale. Airline B withdraws its 5% increased fares. The following week, on Friday, Airline A raises its fares 5% on those city-pairs where Airline B had raised its fares 5% the previous week. On Saturday, Airline B matches Airline A's 5% fare increases, and both Airlines thereafter offer those fares for sale."
2. "Airline A offers for sale a low fare (e.g., $101) for travel on a route that is important to Airline B. Airline B matches the $101 fare for travel on the same city-pair and also offers for sale a $101 fare for travel on a city-pair that is important to Airline A. Airline B withdraws both $101 fares after one day. Airline A then withdraws its initial $101 fare the next day."

Scenario 1 is negotiation by live fares (a counter-offer at half the size, accepted a week later). Scenario 2 is cross-market punishment with a one-day duration.

### 1.4 FareCompare (Rick Seaney) tallies of system-wide domestic hikes

| Year | Attempts | Stuck | Success rate | Source |
|---|---|---|---|---|
| 2007 | 23 | 17 | 74% | FareCompare, Jan 2012 post [V via search excerpt] |
| 2008 | 22 | 15 | 68% | same [V] |
| 2009 | 4 | 4 | 100% | same [V] |
| 2010 | 3 | 3 | 100% | same [V] |
| 2011 | 22 | 9 | 41% | same, and year-end posts [V] |
| 2012 | 15 | 7 | 47% | FareCompare "15th … has Failed" series [V] |
| 2013 (to Sept) | 11 | 3 | 27% | first 10 attempts gave 2 successes, both started by Delta; the 11th (United/Air Canada) stuck [V] |
| 2014 | 21 | 5 | 24% | Seaney byline, Fox News [V] |
| 2015 | 5 | n/a | n/a | Boarding Area post [V] |
| 2016 | at least 5 by mid-year | n/a | n/a | same [V] |

Seaney's own summary: "historically a coin flip". [V] Attempts cluster in fuel-spike years (2007–08, 2011) and vanish in demand slumps (2009–10: few tries, all stuck because nobody tried unless sure). Typical size: up to $10 round trip in 2011–12, $4–$10 across the period. [V for $6 and $10 episodes]

### 1.5 Dated episodes

| Date | Initiator and move | Followers | Outcome | Source |
|---|---|---|---|---|
| 2007 | Big five raise $10 RT | all legacies | Rescinded on routes with discount competitors after Southwest and others "didn't join in" (partial rollback by market) | AviationPros / NBC [V] |
| Mar 2011 | United/Continental file Friday | most carriers at first | Southwest rolled back Sunday; American rolled back Monday morning; failed | AP, 28 Mar 2011 [V via excerpt] |
| Feb 2012 | (unnamed) | broad | "began collapsing over the weekend"; almost fully rolled back by the afternoon | FareCompare [V] |
| Mar 2012 | up to $10 RT | legacies and low-fare carriers | Stuck: third success of five attempts so far that year | NBC, 26 Mar 2012 [V] |
| 2012 | Southwest initiates the year's third attempt | legacies | Stuck: Southwest also leads upward | FareCompare [V] |
| Oct 2012 | United, $10 | others | Southwest first declined, then filed increases on Friday; stuck | [V via excerpt] |
| undated | United, $6 | n/a | Rescinded "to stay competitive with the low-cost carriers, who did not match the fare increase" (United spokeswoman) | TravelMole [V] |
| Feb–Apr 2016 | February hike stuck. In late April Southwest cut $5 each way on tickets bought within 7 days of departure, reversing it | American and United matched the cut **only on routes overlapping Southwest**; Delta raised $5 the same week and did not match the cut | Analysts "troubled"; airline stocks fell | AP (Koenig), 28 Apr 2016 [V] |

### 1.6 Leaders and spoilers

- **Leaders upward:** the largest network carriers of the day: United/Continental, Delta, American, and occasionally Southwest. [V, table above]
- **Spoilers:** Southwest above all, and "low-cost airline support" generally: hikes have "historical difficulty weathering a lack of low-cost airline support" (Seaney). [V] Northwest was the habitual legacy spoiler in the early 2000s [R]; JetBlue and AirTran sometimes played the role [R].
- **Carve-outs:** legacies often structure a hike to exclude markets with LCC nonstop competition, or roll back only there. [V, 2007 episode]
- **Asymmetry:** matching a cut is immediate and close to universal among carriers that overlap; following an increase is conditional on unanimity. The theory is Borenstein's summary of Gertner (1994): with instant matching a price cut pays only for a firm that would prefer every firm to charge the new lower price, and the equilibrium price "will be dictated by the firm that prefers the lowest price". [V]

### 1.7 Later reporting worth reading (paywalled; cited, not relied on)

- Asra Q. Nomani, "Airlines May Be Using a Price-Data Network to Lessen Competition", WSJ, 28 Jun 1990, and "Fare Warning: How Airlines Trade Price Plans", WSJ, 9 Oct 1990: the reporting that preceded the DOJ investigation. [R, titles and dates from memory]
- Scott McCartney, "Airlines Rely on Technology To Manipulate Fare Structure", WSJ, 3 Nov 1997. [R]
- Scott McCartney, "Whatever You Do, Don't Buy an Airline Ticket On …", WSJ Middle Seat, late Jan 2011: most sales launch Monday night, rivals match by Tuesday morning, many sales have expired by Thursday or Friday; Seaney's three-year study put the maximum supply of cheap seats at 3 p.m. ET Tuesday. [V via Consumerist, Neatorama and other excerpts]
- A 2014 Middle Seat follow-up: airline executives "come into work Monday looking to raise fares, not discount them". [V via excerpt]

---

## 2. Signalling, and the DOJ's ATPCO case

### 2.1 Chronology

| Date | Event | Source |
|---|---|---|
| Feb 1982 | Crandall–Putnam call (American/Braniff, DFW) | DOJ complaint [V] |
| 23 Feb 1983 | DOJ sues American and Crandall under Sherman Act §2 (attempted joint monopolization) | complaint [V] |
| 1983–85 | District court dismisses; Fifth Circuit reverses, 743 F.2d 1114 (1984); consent decree | [R for citation]; PBS NewsHour 3 Jul 2015: settled with Crandall keeping a written record of contacts with other airline executives for two years [V] |
| 1990 | WSJ (Nomani) reports on fare signalling through ATPCO | [R] |
| 1991 | DOJ investigation opens | Borenstein [V] |
| 21 Dec 1992 | *US v. Airline Tariff Publishing Co.* filed (D.D.C. 92-2854) against ATPCO, Alaska, American, Continental, Delta, Northwest, TWA, United, USAir. United and USAir settle the same day | Borenstein [V] |
| 1993 | Tunney Act approval of the first decree, 836 F. Supp. 9 | [V] |
| 17 Mar 1994 | Remaining six settle on near-identical terms; 10-year decree (to 2004) | Borenstein; CIS [V] |
| 2010 | Miller (J. Law & Econ.): the case had "at best a temporary effect" on collusion | [V, abstract] |

### 2.2 The Crandall–Putnam call

From the complaint (¶20), Crandall to Putnam: [V]

> "I think it's dumb as hell for Christ's sake, all right, to sit here and pound the shit out of each other and neither one of us making a fucking dime."
> Putnam: "…if you're going to overlay every route of American's on top of … every route that Braniff has -- I can't just sit here and allow you to bury us without giving our best effort."
> Crandall: "Oh sure, but Eastern and Delta do the same thing in Atlanta and have for years."
> Putnam: "Do you have a suggestion for me?"
> Crandall: "Yes. I have a suggestion for you. Raise your goddamn fares twenty percent. I'll raise mine the next morning. … You'll make more money and I will too."
> Putnam: "We can't talk about pricing."

Model-relevant facts in the complaint: two hub carriers at the same slot-constrained airport (DFW slots were cut about 40% in some hours after the 1981 controllers' strike), route-by-route **overlay** as the aggressive capacity move, and a next-morning follow as the natural matching lag. [V]

### 2.3 What DOJ said the airlines did through ATPCO (1988–92)

From the Competitive Impact Statement and Borenstein: [V]

- **Pre-announcement.** A higher fare was filed with a *future first-ticket date*, or a discount was given a *future last-ticket date*. Nothing was sold at the new price while carriers iterated.
- **Iteration and "rolling".** Carriers "iterated back and forth until they reached a point where they were announcing the same fare increase to take effect on the same date". If one carrier held out, the others rolled the effective date forward. A Continental employee: "the actual new levels can be delayed again and again until we have the full cooperation". A DOJ study found **about half of all last-ticket dates in the CRSs turned out to be inaccurate**.
- **Linking across markets.** Shared footnote designators, "empty" footnotes, and fare basis codes (in a few cases containing a rival's two-letter code) tied a fare in one market to a fare in another, so the "trade" was legible.
- **The canonical trade** (Borenstein's Figure 1). Carrier A1 hubs at C1 and flies C1–C3 nonstop. A2 hubs at C2 and undercuts on C1–C3 with a connection. A1 files a fare on **C2–C4** (A2's nonstop route, served by A1 over its own hub) that undercuts A2, gives it a code like A2's offending fare, and attaches a short last-ticket date, "say, two weeks". A1 also files a match of A2's current C2–C4 fare with a first-ticket date equal to that last-ticket date. If A2 "gets the message", it puts the same last-ticket date on its C1–C3 discount. Two weeks later both discounts lapse with no further action. If A2 does not respond, A1 rolls the date and refiles.
- **Scale.** DOJ identified more than fifty agreements, "from New York–Chicago to Klamath Falls–Tampa". One agreement covered about 400 markets; an expert put the consumer cost at $11 million or more on 29 of them.
- **Named episode, April 1989, American v. Delta.** Delta extended American-style discounts to all its Dallas–Chicago flights. American put a last-ticket date on its own fare and filed matching discounts in **Dallas–Atlanta**, Delta's hub market, as a linked offer. Delta restricted its Chicago discounts; American withdrew the Atlanta fares before their last-ticket date. Round-trip prices for many travellers rose by as much as $138. A Delta employee called American's Atlanta fares "obvious retaliation".
- **Each carrier wants high fares on its own hub routes** and is the natural discounter, by connection, on everyone else's. That asymmetry is what makes trades possible.

The airlines' defence is also informative: [V, Borenstein]

- Retaliation was unilateral: a carrier hit where it hurts turns round and hits back where it hurts the other. (Paraphrased.)
- Corporate rebates were secret and seat allocations unobservable, so cheating was possible. DOJ's answer: corporate-discount tickets were under 10% of volume on nearly all routes and were percentage discounts off list.
- Carriers never pre-announced *decreases*.

### 2.4 The decree, and what replaced the banned devices

Prohibited for ten years: disseminating first-ticket dates; last-ticket dates except for sales advertised to consumers (or to match a rival's advertised end date); airline-defined tags, duplicate or empty footnote designators, and fare codes carrying non-fare information. [V]

Not prohibited: filing a bona fide fare, watching, and withdrawing it. The practice moved to weekends (section 1.3). Modern descendants: [V via View from the Wing unless noted]

- **Tactical "flash" fares.** Week ending 22 Feb 2019: American held a $139 Boston–Los Angeles walk-up fare rather than follow an increase; JetBlue filed $139 tactical fares on five routes touching American hubs (BOS–DCA, BOS–DFW, FLL–PHL, JAX–DCA, ORD–FLL). 10 Feb 2020: American sold a $44 Boston–Philadelphia fare outside JetBlue's time band; the JetBlue analyst planned to flash American at the 4 p.m. filing.
- **Executive-to-executive nudges.** 2010: US Airways' Doug Parker forwarded Delta's CEO an email chain complaining that a Delta promotion was "hurting [Delta's] profitability – and unfortunately everyone else's"; Anderson sent it to his lawyer.
- **Earnings-call "capacity discipline".** Aryal, Ciliberto & Leyden: when all legacy carriers in a market discuss capacity discipline on their calls, offered seats are about **2% lower**; the effect appears only when they communicate at the same time. [V, arXiv abstract]

---

## 3. Named fare wars and their P&L effects

### 3.1 United States, 1977–1995

| Dates | Episode | How the move was made | Response and speed | Result |
|---|---|---|---|---|
| 26 Sep 1977 – 5 Feb 1982 | **Laker Skytrain**, Gatwick–New York | Walk-up, no-frills transatlantic; from Oct 1980 "Super Economy" at about half incumbents' economy | Incumbents introduced matching standby/budget fares in 1977–78 [R]. **Oct 1981: Pan Am cut its lowest economy fares on Laker routes by up to 66%**; BA and TWA followed after the winter peak. Laker's loads and cash inflow halved between Oct 1981 and Feb 1982. BCal and other DC-10 operators told McDonnell Douglas and GE they would take their business elsewhere if a rescue went ahead | Collapsed 5 Feb 1982, debts £270m. Antitrust suit against BA, BCal, Pan Am, TWA, Lufthansa, Air France, Swissair, KLM, SAS, Sabena, Alitalia, UTA: settled out of court for US$50m; BA paid Laker £8m personally; July 1985 BA added $35m. [V, Wikipedia citing *Flight International*] Often reported as about $48m to creditors plus $8m to Laker [R] |
| 30 Apr 1981 – 1 Feb 1987 | **People Express** | Newark base, unbundled, flat low fares; RPKs 650m (1981) to 17,638m (1985); 72 aircraft and 41 cities by Aug 1985 | **17–18 Jan 1985: American's "Ultimate Super Saver"**: fares at or below People's, non-refundable, advance-purchase, capacity-controlled per flight by yield management [V]; 30-day advance and up to about 70% off [R]. American filled otherwise-empty seats at People's prices while protecting late-booking full fares; People had no system to answer | Burr: "We were a vibrant, profitable company from 1981 to 1985, and then we tipped right over into losing $50 million a month" [V, secondary]. Also the Frontier purchase (1985) and its debt. Sold to Texas Air, Sept 1986, about $125m; folded into Continental 1 Feb 1987 [V] |
| 9 Apr 1992 – autumn 1992 | **American "Value Pricing"** | Announced on or about 9 Apr 1992 [V, 824 F. Supp. 689]. Four fare levels [V]: first class, unrestricted coach, 7-day and 21-day advance purchase; unrestricted coach cut about 38%, first class 20–50%; most negotiated discounts ended [R] | Most majors matched within days; TWA, USAir, Continental and America West undercut by 10–20% [R]. **Late May: Northwest's family promotion**, an adult free with a paying child, which then "grew up" into 50% off for everyone [V, CS Monitor excerpt of 2 Jun 1992]; dated 26 May and branded "Grown-Ups Fly Free" [R]. **American answered with cuts of up to 50%** [V via excerpt]: on 27 May, purchase by 5 June, travel to mid-September, matched industry-wide within a day, reservation systems swamped [R] | American abandoned the structure in the autumn [R]. Industry net loss 1992 about $4.8bn [R, ATA]; "record losses in 1990, 1991, and 1992" [V, Borenstein]. Continental and Northwest sued for predation (S.D. Tex., Galveston; opinion of 7 Jun 1993, 824 F. Supp. 689) [V]; that opinion let the case go to trial and a jury found for American in Aug 1993 [R]. Crandall to Bloomberg, July 1992: the market must be allowed to finish eliminating surplus capacity [V via excerpt] |
| Oct 1993 – Apr 1995 | **Continental Lite** | Airline within an airline; about one-third of Continental's capacity; 45 cities; Greensboro hub at 83 daily flights | Built to answer Southwest and the start-ups | Losses reported at $140m to $300m; dropped April 1995 (Adam Bryant, "Continental Is Dropping 'Lite' Service", NYT, 14 Apr 1995) [V, Wikipedia] |

**Lessons.**

1. The 1992 war started as an attempt at price *leadership through simplification*. It failed because (a) weaker carriers undercut the new structure and (b) one tactical promotion by Northwest triggered a full-depth, industry-wide match within 24 hours. A single 50%-off sale with a 10-day purchase window sold most of a summer's leisure inventory.
2. People Express is the template for **segmented matching**: match the entrant's price only on the seats the entrant could have won, using restrictions and inventory control.
3. Laker shows the long-haul version: incumbents waited four years, then cut up to 66% when the entrant was financially stretched (dollar debt, falling sterling), and also worked on the entrant's suppliers and lenders.

### 3.2 Southwest's entries and the "Southwest effect"

| Evidence | Finding | Source |
|---|---|---|
| Bennett & Craun, DOT Office of Aviation Analysis, May 1993 | In three California markets, after Southwest entered, average fares fell about 50% and traffic more than tripled. Oakland–Ontario: fares −60%, traffic +300% (p. 7). Another summary gives −48% fares, +200% traffic. The nearest competitor on cost, America West, had stage-length-adjusted unit costs 20% above Southwest | secondary summaries [V] |
| Morrison (JTEP 2001) | Savings from Southwest's actual, adjacent and potential competition in 1998: **$12.9bn**: $3.4bn direct, $9.5bn through other carriers' fares | [V] |
| Goolsbee & Syverson (QJE 2008) | 704 routes threatened when Southwest gained both endpoints; the probability of entry rises about 70-fold. Incumbent fares start falling about 3 quarters before; **−17% by the quarter Southwest has both endpoints, −21% when it starts flying, −24% to −29% later**. "More than half" the total effect precedes entry. No significant rise in seats or flights; load factors rise. No cuts on routes from nearby non-Southwest airports. On 223 routes where entry was pre-announced and certain, no significant pre-emptive cut | [V, NBER w11072] |
| Brueckner, Lee & Singer (2013; data YE 2008-Q2) | Southwest nonstop on the airport pair: **−27% to −29%**; on an adjacent pair: −13% to −17%; as potential entrant: −9% to −10%. Other LCC nonstop: −13% to −18%; adjacent: −5% to −6%; potential: 0 to −4%. In 2000 data the Southwest nonstop effect was −46% | IATA briefing table [V] |
| Wittman & Swelbar (MIT 2013) | Southwest's average one-way fare effect fell from $36 (2007) to $17 (2012). 2012: JetBlue $32, Allegiant $29, Spirit $22 | [V, Wikipedia] |
| Beckenstein & Campbell (UVA/Campbell-Hill 2017) | 109 markets entered 2012–15: demand +28% on average; smaller than 1993; about $9.1bn a year in savings | [V via excerpts] |

Named contests:

- **America West, Phoenix.** The only incumbent near Southwest's costs (still +20%). Coexisted by matching rather than fighting. [V for cost gap; R for the rest]
- **USAir at Baltimore.** Southwest entered BWI in September 1993; USAir matched fares, then shrank [R]. USAir's answer was **MetroJet**: launched 1 Jun 1998 at BWI to "head off burgeoning competition from low-cost carriers", closed after 11 Sep 2001 with old 737-200s, mainline labour costs and "cutthroat competition from Southwest at BWI". [V]
- **US Airways at Philadelphia.** Southwest entered 9 May 2004. CEO David Siegel told staff Southwest was coming "to kill us" [R, widely reported]. US Airways capped walk-up fares and expanded low fares in PHL markets [R]. It was bankrupt again in Sept 2004 and merged with America West in 2005.
- **Legend Airlines, Love Field.** Legend started 5 Apr 2000 with 56-seat all-business DC-9s to IAD, LAS, LAX and LGA at about coach fares. American, after litigating to block it, reconfigured Fokker 100s to 56 seats and **opened its own Love Field service on 1 May 2000 (26 days later)** to ORD, LAX and LGA. Legend lost about $25m in six months and suspended flying on 3 Dec 2000. [V, Wikipedia citing *Dallas Morning News*]
- **Love Field after the Wright Amendment** (13 Oct 2014). Southwest went long-haul from Love; Virgin America moved in with two gates (3.58% share in month one). Love Field enplanements rose from 8.4m (2013) to 14.5m (2015). [V] American matched on Dallas routes and blamed Dallas for much of its 2015 unit-revenue decline; Spirit was squeezed at DFW [R].

### 3.3 JetBlue, Independence Air, Virgin America, and the airline-within-an-airline

| Dates | Entrant or response | Facts | Source |
|---|---|---|---|
| 11 Feb 2000 | JetBlue starts at JFK with 75 slots awarded Sept 1999 and $128m of start-up capital | Profitable through the post-9/11 slump; first quarterly loss Q4 2005 | [V] |
| 15 Apr 2003 – 30 Apr 2006 | **Song** (Delta) | 48 Boeing 757s, 200+ daily flights, Northeast–Florida against JetBlue. Ended in Delta's bankruptcy: could not carry two brands; union friction | [V] |
| 12 Feb 2004 – 6 Jan 2009 | **Ted** (United) | 56 all-economy A320s; Denver base against Frontier, also ORD, SFO, IAD. Ended in the 2008 fuel spike | [V] |
| 1 Jun 1998 – Dec 2001 | **MetroJet** (US Airways) | see 3.2 | [V] |
| Oct 1996 – 2003; Oct 1994 – 2001 | **Delta Express; Shuttle by United** | Same pattern | [R] |
| 16 Jun 2004 – 5 Jan 2006 | **Independence Air** at Dulles (the former United Express partner ACA) | Fares from $29 one-way; about 1m new Dulles passengers in three months. Legacies **matched fares**; United used Mileage Plus bonuses, including free trips, to hold business travellers. Flights "far below capacity" by Oct 2004. Chapter 11 on 7 Nov 2005; closed 5 Jan 2006 with 200 daily departures to 37 cities. Lost almost $150m | Wikipedia citing Joe Sharkey, "Independence Air Ends: No Bang, Some Whimpers", NYT, 10 Jan 2006 [V] |
| 8 Aug 2007 | **Virgin America** | First profit Q3 2010; 2012 net loss $145.4m; sold to Alaska for $2.6bn equity (announced 4 Apr 2016, closed 14 Dec 2016). Transcontinental incumbents matched fares and upgraded product [R] | [V] |

**Why the sub-brands failed.** [V for facts; the synthesis is mine]

- They ran at the parent's labour and overhead cost: MetroJet's "labor costs were high"; Continental's costs were already low and Lite still lost money.
- They cannibalised and confused the main brand.
- They were first to go in a cash crisis (9/11, bankruptcy, the 2008 fuel spike).
- Lifespans: 1.5 years (Lite), 3 (Song), 3.5 (MetroJet), 5 (Ted). The survivors worldwide are subsidiaries with separate certificates, labour and management (Jetstar, Eurowings, Level, Scoot).

### 3.4 ULCC growth, 2013–19, and the basic-economy response

- Spirit: revenue $1,654m to $3,831m, fleet 54 to 145, passengers 12.4m to 34.5m between 2013 and 2019. [V, Wikipedia from annual reports]
- **Total revenue per passenger, computed from that table: $133 (2013), $135 (2014), $120 (2015), $107 (2016), $109 (2017), $113 (2018), $111 (2019).** That is −11.5% in 2015 and −20.5% over two years, coinciding with the majors' decision to match. Net profit still rose in 2015 ($317m) because fuel halved. Ben Baldanza was "abruptly replaced" by Robert Fornaro on 5 Jan 2016. [V for numbers and the CEO change; the causal reading is mine]
- Delta introduced basic economy in 2012; "by the end of the decade all U.S. legacy carriers had similar products", and many passengers shifted to the legacies when basic economy was competitively priced. [V, Wikipedia] The first Delta markets were Detroit–Florida routes overlapping Spirit; United announced its version in Nov 2016 and American in Jan 2017 [R].
- American, 2015: management said it would match ULCC fares wherever they overlapped, noting that about 87% of its customers flew it once a year or less yet produced about half its revenue (Scott Kirby) [R].
- United, 2016–18: Kirby moved to United in Aug 2016 and matched ULCC fares out of its hubs through 2017. Spirit's Fornaro complained publicly of aggressive pricing in Chicago, Houston, Newark and Denver; United's Q3 2017 call sent its shares down about 12%; on 23 Jan 2018 United guided to 4–6% annual capacity growth to 2020, its shares fell about 11%, and the sector sold off on fare-war fears [R].
- AP, 28 Apr 2016: some analysts blamed the major airlines for matching the fares of ultra-low-cost carriers such as Spirit and Frontier; average fares had been dipping since early 2015. [V]
- Spirit shut down in May 2026 (N. Chokshi, "Spirit Airlines, a Pioneer of Low-Fare Flights, Shuts Down", NYT, 2 May 2026). [V, Wikipedia]

**Lesson.** A product-level match (same base fare, a stripped fare class, small inventory) lets a network carrier meet a ULCC price without repricing its whole cabin. It costs the ULCC 10–20% of unit revenue within two years and costs the legacy much less. It works only where the legacy has frequency and loyalty advantages.

### 3.5 Europe

| Dates | Episode | Facts | Source |
|---|---|---|---|
| 1986 | **Ryanair v. Aer Lingus/BA, Dublin–London** | Ryanair launched Dublin–Luton at £99 return, about half the incumbents' fare (the HBS case gives I£98 against I£208 [R]). **Aer Lingus cut to £95; Ryanair went to £94.99.** 82,000 passengers in the first year. Barrett's studies: fares −54%, traffic +92% by Aug 1987 [R] | Wikipedia [V] |
| 1987–90 | Ryanair's near-failure | Lost £3m in 1987; £20m accumulated by 1990. Relaunched on the Southwest model: 19 destinations cut to 6, cheapest return £99 to £59, single fleet type. 945,000 passengers in 1992, 3.7m in 1997 | [V] |
| 22 May 1998 – 2002 | **Go** (BA) | Stansted base against Ryanair and easyJet; easyJet sued (cross-subsidy). Strategy change June 1999 "to stem mounting losses". Sold to 3i and management for £100m (June 2001), then to easyJet for £374m (16 May 2002). Go left Dublin in Dec 2001, attributed to Ryanair | [V] |
| 4 Jan 2000 – Oct 2004 | **Buzz** (KLM uk) | Stansted. Sold to Ryanair, announced 31 Jan 2003, closed 10 Apr 2003, about €20m; Ryanair dropped routes and shut it 31 Oct 2004 | [V] |
| 23 Mar 2002 – 9 Sep 2012 | **bmibaby** (bmi) | East Midlands. Fleet cut 17 to 12 in 2009 "to stem record losses"; closed by IAG, which found no buyer | [V] |
| 12 Nov 2001 – 2002 | **Lufthansa v. Germania** | section 4.5 | [V] |

Flag-carrier sub-brands in Europe were sold or closed within 3–10 years. The incumbents' lasting response was to cede short-haul leisure flows from secondary airports and defend hubs, slots and loyalty.

### 3.6 India and Asia-Pacific

- **Air Deccan** (2003) brought Re 1 promotional base fares [V for the Re 1 reference]. Kingfisher (launched 9 May 2005) took control in 2007–08 and rebranded it Kingfisher Red, which "failed to stem losses" and closed in 2011. Kingfisher lost over ₹10bn a year for three years, had accumulated losses above ₹70bn by early 2012, and lost its licence on 20 Oct 2012. [V]
- **SpiceJet**, FY2013-14: net loss ₹1,003 crore on ₹6,304 crore revenue. FY2014-15: loss ₹687 crore; fleet cut from 58 to 34; control passed to Ajay Singh in Jan 2015. [V] The 2014 collapse followed a year of flash sales (Re 1 and ₹499 base fares) that IndiGo, GoAir, Jet and Air India matched within a day and that the DGCA criticised; oil companies cut off fuel in mid-December 2014 [R].
- **Tigerair Australia** (first flight 23 Nov 2007). Two days before launch Jetstar sold 5,000 seats at five cents on seven routes; it promised to "double the difference" on any cheaper rival fare. Qantas declined to supply ground handling at Alice Springs, delaying Tiger there by three months. Grounded by CASA from 2 Jul to 12 Aug 2011, losing about $4.2m a week. 60% sold to Virgin Australia for A$35m (announced Oct 2012); the rest for A$1 (Oct 2014). [V]
- **Qantas v. Virgin Australia, 2011–14:** section 4.7.
- South-east Asia (AirAsia v. Malaysia Airlines from 2002, Lion Air, Cebu Pacific, the Thai overcapacity of 2014–19): the same shape, with state-owned flag carriers holding capacity through losses [R, not researched].

### 3.7 Transatlantic long-haul low-cost, 2013–21

| Date | Event | Source |
|---|---|---|
| 30 May 2013 | Norwegian Long Haul starts Oslo–JFK; 787s from Aug 2013; 34 787-9s on order by May 2016; one-way fares advertised at $174–$199 in 2017 | [V] |
| 2015–18 | WOW air: capacity about 740,000 passengers (2015) to over 1.6m (2016); 36 destinations by 2018; a loss in 2017 after two profitable years | [V] |
| 15 Mar 2017 | IAG announces **Level**, "a year ahead of schedule", explicitly a response to long-haul LCCs; flying from Barcelona June 2017 on Iberia's certificate; 52,000 seats sold in two days | [V] |
| Apr 2018 | Basic economy crosses the Atlantic (Delta/Air France-KLM/Virgin; American/BA/Iberia/Finnair; United and Lufthansa Group later that year). IAG takes about 4.6% of Norwegian and makes two approaches, both rejected; sells in early 2019 | [R] |
| 28 Mar 2019 | WOW air stops flying after Icelandair (Nov 2018, Mar 2019) and Indigo Partners walk away | [V] |
| 14 Jan 2021 | Norwegian ends all long-haul | [V] |

Pattern: joint-venture incumbents matched the headline price with an unbundled fare class on the same aircraft, started a low-cost subsidiary within about four years of entry, and waited for the entrant's balance sheet to fail.

---

## 4. Predation cases

### 4.1 *United States v. AMR Corp.* (D. Kan. 2001; 10th Cir. 2003)

**Setting.** American carried 70% of nonstop DFW passengers and 77% of DFW-originating nonstop passengers; Delta 16%; nobody else above 4%. [V, complaint ¶20]

**The "DFW LCC Strategy".** [V, complaint ¶¶25–31]

- 1993: American estimated $3.6bn of its revenue "at risk" from LCCs and potential annual losses of $586m to $1.47bn.
- It put ValuJet's effect on Delta in Atlanta at $232m of annual revenue: "clearly we don't want that to happen to [American] at DFW".
- February 1996, senior management: if an LCC looks viable under American's normal profit-maximising response, saturate the route with capacity at low fares, match the LCC's *connecting* fares with American's nonstops, watch its finances, break-even load factor and gate head-counts.
- Crandall (1996): "If you are not going to get them out then no point to diminish profit."
- Against Southwest at Love Field American did **not** do this: it "set fares and capacity so as to maximize its profits on the assumption that it would have to compete with Southwest over the long term".
- American's normal discipline: senior management reviewed, monthly, every route that was negative on FAUDNC (fully allocated earnings plus upline/downline contribution net of costs) over the trailing twelve months, and prescribed changes "which may include reducing service or exiting the route altogether". The LCC routes were exempted.

**Route record.** [V, complaint ¶¶32–47]

| Route / LCC | Before | American's moves | LCC's fate | After exit |
|---|---|---|---|---|
| DFW–Kansas City / Vanguard, round 1 | Jan 1995: Vanguard 3 daily round trips. American 8, 65% share, average one-way fare $108. Delta 6 | Matched Vanguard's fares on all flights: average fare $80 by Apr 1995. Delta left May 1995. **May–Jul 1995: +6 round trips (8 to 14)**, intended "to drive [Vanguard] from the market" | Vanguard cut to 1 round trip in Apr 1995; left the nonstop in Dec 1995 | Frequency 14 to 11 (Feb 1996) to 10 (Jul); fares $112–$147, up to 80% higher. From "worst performer" to "best in the west" |
| DFW–Wichita, Kansas City, Phoenix, Cincinnati / Vanguard, round 2 | Sept 1996: Vanguard announces a DFW build-up from 1 Oct (nonstops to Kansas City, Phoenix, Cincinnati; existing Wichita; connections beyond Kansas City) | "Within days": Kansas City +2 round trips pulled forward from Nov to 1 Oct, +1 on 1 Nov (13 in all). **Wichita: 5 jet round trips replace 4 commuter flights, seats +35%.** **Cincinnati: re-entered 1 Dec with 3 round trips**, a route dropped as unprofitable in 1994. Phoenix: seasonal additions pulled forward a month or more. Matched Vanguard's connecting fares on DFW–Chicago and DFW–Des Moines with nonstops, on flights that "bracketed" Vanguard's times | Vanguard left Phoenix and Cincinnati in Nov 1996, Wichita in Dec 1996 | Wichita by June 1997: seats −30%, back to the old level; average one-way fare from about $60 to over $90 (+50%). Phoenix: fares had fallen about 30%, returned "quickly". Cincinnati: fares +60–80% after the first month |
| DFW–Long Beach / Sun Jet | Sun Jet at DFW since 1994 (Newark, Long Beach, Tampa). American's stance "moderate" unless Sun Jet "adds frequencies … or adds new DFW spokes" | Oct 1996 Sun Jet adds a third Long Beach round trip and announces Oakland. Mid-Nov: American removes restrictions on Newark and Tampa fares and matches Oakland. **31 Jan 1997: re-enters Long Beach with 3 round trips** (dropped 1994), matching fares; a 4th in Aug 1997, despite diverting its own LAX-area traffic | Oakland never started; Sun Jet left Tampa Mar 1997, Newark Dec 1997, Long Beach Jan 1998 | Fares +30% and more within two months |
| DFW–Colorado Springs / Western Pacific | Apr 1995: Western Pacific 2 round trips. American 5 (100-seaters), fare above $150. Delta 3 | Jul 1995: +2 round trips and a fare match **on flights at the same time of day**. End-1995: Western Pacific "pretty much in check with enplaned load factors of 30.1%"; it drops to 1 round trip in Jan 1996. Feb 1996: escalate anyway: "get [Western Pacific] out before they are encouraged to put [the second] frequency back". Apr 1996: more low-fare seats near its times. Jun 1996: an 8th round trip and larger aircraft. **Sept 1996: match on all flights and override yield management so low fares are always available**; hold summer frequency through winter. Mar 1997: Western Pacific goes to 3, American to 9 with larger aircraft | Left 15 Oct 1997; later bankrupt | Sept 1996 – Jun 1997 fares $81–$105. By Feb 1998: 6 round trips, average fare $137 |

**Law.** [V, 335 F.3d 1109]

- Filed 13 May 1999. Summary judgment for American (140 F. Supp. 2d 1141, D. Kan. 2001); affirmed 3 Jul 2003.
- The government offered four tests of whether the *added capacity* lost money, built on American's own FAUDNC, VAUDNC and VAUDNC-AC measures. The court held all four "invalid as a matter of law, fatally flawed in their application, and fundamentally unreliable": they included fixed or arbitrarily allocated costs, or counted forgone profit as cost.
- "It is uncontested that American did not price below AVC for any route as a whole."
- The district court noted that American's prices "only matched, and never undercut" the LCCs. The appeals court declined to make "meeting competition" a formal §2 defence.
- On DFW–Wichita, fares ran $99–108 before, $58–61 during and $100–123 after.

**For the model.** Under US law the incumbent's safe harbour is: match, never undercut, and add capacity as long as route-level revenue covers route-level variable cost. That is the legal envelope for a "fortress hub" archetype.

### 4.2 *Spirit Airlines v. Northwest Airlines* (6th Cir. 2005)

[V, 431 F.3d 917]

| | Detroit–Philadelphia | Detroit–Boston |
|---|---|---|
| Northwest before | About 70–72% share, 6 daily round trips; lowest unrestricted $355, lowest restricted $125 each way. US Airways 4 round trips, about 27% | 89% share, 8.5 round trips; lowest unrestricted $411, restricted $189. A 13.7% summer seat cut was planned (3,753 to 3,238) |
| Spirit's entry | 15 Dec 1995, one 87-seat DC-9 round trip at $49. Load factor 64% (Jan), 88.5% (June). Second flight added 28 Jun 1996 | 15 Apr 1996, one round trip at $69/$89/$109 |
| Northwest's response | **None for six months.** On 19 Jun 1996, nine days before Spirit's second flight, it cut its lowest fares, unrestricted included, to $49 on every flight | **Same day:** $69 on every flight; round trips up from 8.5 to 10.5, including a 289-seat DC-10, about three times Spirit's daily seats on the route |
| Depth of the match | 40.5% of Northwest passengers paid $49 or less in Jul–Sep 1996 | 74.5% paid $69 or less, Apr–Sep 1996. Northwest's fare was below Spirit's lowest on 93.9% of days |
| Spirit's load factor | 43% (Jul), 36% (Aug), 31% (Sep) | 18%, 21%, 24%, 31%, 29%, 17% (Apr–Sep) |
| Exit | Second flight dropped by 20 Aug; left 29–30 Sep 1996 | Left Sep 1996 |
| After | Northwest's lowest unrestricted fare: $271, then $461; $279 on 28 Oct 1996; $416 by Apr 1998 | not given |

Northwest controlled 64 of Detroit's gates on long leases and carried 78% of Detroit passengers. Spirit paid $100,000 to add the Philadelphia flight and a 25% landing-fee premium as a non-signatory. Experts disagreed on cost: Spirit's put Northwest's net revenue $7–$12 below average variable cost per low-fare passenger; Northwest's found positive contribution every month. The court reversed summary judgment (a leisure-traveller market was triable); the Supreme Court declined review in 2006. [V]

**For the model.** The trigger was not entry. It was the entrant reaching a high load factor and adding a second frequency. A single daily flight by a small carrier was ignored for six months.

### 4.3 Northwest v. Reno Air (1993)

Reno Air started Reno–Minneapolis in 1993. Northwest began its own Minneapolis–Reno service and **opened a mini-hub at Reno overlaying much of Reno Air's network**. DOJ called it "probably our best known airline predation investigation"; DOT intervened and Northwest dropped the overlay. Reno Air still left Minneapolis within months; one-way fares then went from about $200 to about $400. Reno Air's 1997 suit was dismissed in April 1999. [V, DOJ speech, MPR and Las Vegas Sun excerpts]

### 4.4 DOT's 1998 "unfair exclusionary conduct" proposal

Proposed 10 Apr 1998 (63 FR 17919, docket OST-98-3713), drafted with DOJ; comment period extended to Sept 1998; never adopted. In January 2001 DOT published findings and the Oster–Strong report, "Predatory Practices in the U.S. Airline Industry" (15 Jan 2001), which concluded that predation "may have occurred in the past" and that a price-cost standard may miss part of it. DOT said it would proceed case by case. [V]

The proposal's three presumptive triggers, when a major at its hub responds to a low-fare entrant: [R]

1. It adds capacity and sells so many seats at very low fares that its local revenue is lower than under a reasonable alternative response.
2. The number of local passengers it carries at the entrant's fare level exceeds the entrant's total seat capacity.
3. The number of local passengers it carries at those fares exceeds the number of low-fare passengers the entrant carries.

All three are counts of passengers carried at the matched fare relative to the entrant's capacity. That makes a clean simulator rule for a strict-regulator regime.

DOT's "Dominated Hub Fares" (Jan 2001): passengers in hub markets without low-fare competition paid about 41% more than those in comparable markets with it, 54% more on short-haul routes [R].

### 4.5 Lufthansa v. Germania (Bundeskartellamt, 2002)

[V, Bundeskartellamt releases of 22 Jan, 19 Feb and 10 Apr 2002]

- 12 Nov 2001: Germania starts Berlin Tegel–Frankfurt at **€99 one-way, fully flexible**.
- Lufthansa's fully flexible economy fare had been €485. It introduced a €200 return bookable as separate one-ways (**€100 each way, a cut of almost 60%**), including catering and miles, with about three times the frequency. It offered nothing like it on any other route.
- Germania cut to €55, could not sustain it, and returned to €99 at the start of 2002. Lufthansa moved to about €105 on 1 Jan 2002.
- 19 Feb 2002 order, immediately enforceable: Lufthansa must price **at least €35 above Germania's one-way fare**, capped so the required fare never exceeds €134, **for two years**. The Office valued Lufthansa's service advantage at about €35 one-way and cited earlier episodes (Munich–Stansted against Go, Munich–Frankfurt against Deutsche BA) where Lufthansa raised prices after the rival left.
- Düsseldorf Higher Regional Court, Apr 2002: provisionally upheld, with the gap reduced to €30.50 (a €129.50 fare).

**For the model.** A regulator can impose a minimum price differential equal to the incumbent's quality premium for a fixed term. The €35 on a €99 fare (about 35%) is also a rare official estimate of what frequency, loyalty and service are worth on a 1-hour business route.

### 4.6 Air Canada v. WestJet/CanJet (Competition Tribunal, 2003)

[V, Tribunal materials and commentary via excerpts]

- Commissioner's application 2001 (CT-2001-002) under section 79 of the Competition Act, over Air Canada's responses to WestJet and CanJet in eastern Canada in 2000–01. Phase I test routes: **Toronto–Moncton** (WestJet) and **Halifax–Montreal** (CanJet).
- Decision 22 Jul 2003: Air Canada operated **below avoidable cost** on the test routes. The Tribunal treated a wide range of costs, including overhead, as avoidable over the period: far broader than US average variable cost.
- Stayed by Air Canada's creditor protection (1 Apr 2003); Phase II never ran; the Commissioner closed the litigation on 29 Oct 2004.
- The Bureau opened a predation inquiry into WestJet and its ULCC arm Swoop in Dec 2018.

### 4.7 Australia: Compass, and Qantas's 65% line

- **Compass Mk I** (1 Dec 1990 – 20 Dec 1991): A300s on trunk routes; given "the least accessible aircraft parking bays inside a competitor's terminal"; failure attributed partly to "sustained fare discounting by its competitors". **Compass Mk II**: 31 Aug 1992 – 11 Mar 1993. [V, Wikipedia] The Trade Practices Commission's review did not find unlawful predation; terminal access was the structural problem [R].
- **Qantas Group's 65% domestic capacity share.**
  - 4 May 2012: Alan Joyce called the 65% share "profit maximising": "We intend to retain the market share that enables us to maximise profit." Qantas added peak services on Sydney, Melbourne and Brisbane routes, put 747s on Sydney–Perth and added A330s to Western Australia. [V, Travel Weekly]
  - 2013: Virgin's John Borghetti characterised the policy as "if a competitor puts one [plane] in, we'll put two in as a group" and asked for it to be investigated. [V via excerpt]
  - December-half domestic EBIT: Qantas $328m to $218m; Virgin $87m to $49.3m (−43%). [V via NZ Herald excerpt; the half-year is probably the one to Dec 2012]
  - 5 Mar 2014: Joyce would keep the strategy; Virgin had added 18% capacity in two and a half years and Qantas "half that"; Virgin was "proportionally … losing more money than Qantas". [V, SBS]
  - 21 Mar 2014: CAPA called the 65% share a "line in the sand"; additions had "shredded profitability by hundreds of millions of dollars"; about a third of domestic capacity touched Western Australia, where intra-state capacity grew nearly 40% in two years. [V]
  - May 2014: Qantas guided to zero domestic capacity growth for the first quarter of FY2015; Joyce said the 65% target had been "misinterpreted". [V via excerpt]
  - FY2014: Qantas statutory loss about A$2.8bn (including a fleet write-down of about A$2.6bn); Virgin Australia loss about A$356m [R].

**For the model.** A stated capacity-share floor is a credible commitment only while the incumbent can fund it. It lasted about three years and ended when both sides lost money in the same year.

### 4.8 Predation summary

| Case | Incumbent share | Price move | Capacity move | Lag to respond | Entrant exit after response | Post-exit fare | Legal result |
|---|---|---|---|---|---|---|---|
| American / Vanguard, Kansas City 1995 | 65% | match, −26% | +75% frequency | price at once; capacity after 4–6 months | 11 months from entry, 5–7 after the capacity add | up to +80% within 6 months | no liability |
| American / Vanguard, Wichita 1996 | high | match, about −40% | +35% seats | days after announcement | about 3 months | +50% by 6 months | no liability |
| American / Sun Jet, Long Beach 1997 | re-entrant | match | re-entry, 3–4 round trips | 3.5 months | 12 months | +30% in 2 months | no liability |
| American / Western Pacific, Colorado Springs | high | match, from $150 to $81–105 | 5 to 9 round trips, larger aircraft | 3 months, then repeated escalation | 30 months from entry | $137 | no liability |
| Northwest / Spirit, Boston 1996 | 89% | from $189–411 to $69 on all flights | +24% frequency plus widebody, reversing a planned cut | 0 days | 5.5 months | n/a | triable |
| Northwest / Spirit, Philadelphia 1996 | 72% | from $125–355 to $49 on all flights | none | 6 months, triggered by a second flight | 3.3 months | $271–461 | triable |
| Northwest / Reno Air 1993 | hub | match | new route and a hub overlay | weeks | months | about +100% | agency pressure |
| Lufthansa / Germania 2001 | dominant | €485 to €100, at the entrant's price with a better product | none | weeks | survived under the order | n/a | price-gap order |
| Air Canada / WestJet, CanJet 2000 | dominant | match | added | n/a | n/a | n/a | below avoidable cost |
| American / Legend 2000 | DFW 70% | n/a | cloned the product at the entrant's airport | 26 days | 7 months | n/a | none |

---

## 5. Structural findings from IO economics, as parameters

| Topic | Finding | Suggested use | Source |
|---|---|---|---|
| Hub premium | GAO (RCED-90-102, July 1990): fares at 15 concentrated airports against 38 unconcentrated ones [V for the design]; about 27% higher [R]. Borenstein (RAND 1989): the premium accrues mainly to the dominant carrier itself; rivals on the same routes do not enjoy it [R]. Evans & Kessides (REStat 1993): airport dominance matters, route share adds little once it is controlled for [R]. "Scholars have shown the 'hub premium' to be between twenty and forty percent" [V, secondary]. Across the fare distribution: under 10% at the 10th percentile, almost 25% at the 90th [V, MPRA paper excerpt]. Borenstein (2011): the legacy premium over LCCs "shrinking by more than 60%" over 20 years [V] | Dominant-carrier markup at its own hub, on local origin traffic: +20–25% in a 1990 world, +5–10% once LCCs have about 30% of the market. Apply mainly to the top of the fare ladder | as cited |
| What sustains it | FFPs, corporate contracts, override commissions, gate control. Ciliberto & Williams instrument multimarket contact with exclusive or preferential gate leases. Lederman (2008): FFP partnerships explain part of it [R] | section 6 | [V, R] |
| Multimarket contact | Evans & Kessides (QJE 1994), 1,000 largest city pairs, 1984–88: moving from the 25th to the 75th percentile of contact raises fares **about 5%**. Ciliberto & Williams replicate at **about 3%**, range 1–5% (up to 8.5% instrumented). Structurally: carriers with little contact do not cooperate; carriers with extensive contact sustain "almost perfect coordination"; marginal contact matters only at low to moderate levels; assuming Bertrand–Nash overstates marginal cost by about 30% | Tacit-cooperation weight per rival pair, a saturating function of overlapping markets | [V, CW working paper] |
| Entry | Berry (Econometrica 1992): profitability falls with the number of entrants; presence at both endpoints is the main predictor [R]. Goolsbee & Syverson: both-endpoint presence raises a route's entry probability about 70-fold [V] | Entry candidates are routes where the carrier already serves both endpoints | as cited |
| Threat of entry | −17% before entry, −21% at entry, −24% to −29% after; starts about 3 quarters ahead; no capacity response; only on the threatened route | Pre-emptive price response to a strong LCC | [V] |
| Number and type of competitors | Brueckner, Lee & Singer, YE 2008-Q2. Legacy nonstop competitors 2 to 1: 0 to +3.9% (insignificant where LCCs hold 45% or more of the city pair); 3 to 2: none; legacy on an adjacent airport: none. Southwest nonstop −27% to −29%; other LCC nonstop −13% to −18%; Southwest adjacent −13% to −17%; other LCC adjacent −5% to −6%; Southwest potential −9% to −10%; other LCC potential 0 to −4%. **In 2000 data:** 2 to 1 +11.8%; 3 to 2 +4.1%; legacy adjacent −5.4%; Southwest nonstop −46%; Southwest adjacent −39%; Southwest potential −18%; other LCC nonstop −27%. Older studies: 2 to 1 +7–8%, 3 to 2 +4–6% | The core calibration table for route fare levels. The legacy effect depends on era and LCC share | [V, IATA briefing] |
| Price dispersion | Borenstein & Rose (JPE 1994): expected absolute fare difference between two passengers on the same carrier and route is 36% of the mean fare; dispersion *rises* with competition in cross-section. Gerardi & Shapiro (JPE 2009), panel 1993–2006: average Gini 0.22 (expected difference 44% of the mean); competition *reduces* dispersion; on big-city routes the 90th-percentile fare falls about twice as much as the 10th when competition rises; an LCC's presence cuts a legacy's dispersion by 2–5% | Competition compresses the fare ladder from the top. Monopoly routes keep a wide ladder; LCC entry pulls walk-up fares down more than advance-purchase fares | [V] |
| S-curve | Wei & Hansen (TR-E 2005; nested logit; 13 nonstop duopoly markets, 1989–99): support for an S-curve; frequency buys more share than aircraft size. Button & Drexler (2005): some S-curve in the early 1990s, close to linear by the early 2000s (as summarised in an MIT paper on frequency competition) | Share proportional to frequency share to the power α, α = 1.0–1.3. Classical planning values 1.3–1.7 [R]. Make α higher for business-heavy short-haul and lower where LCCs and online shopping dominate. ESTIMATE | [V] |
| Mergers | Kim & Singal (AER 1993): 27 mergers, 1985–88; fares rose on merging firms' routes relative to controls [V]: +9.44% for the merging firms and +12.17% for rivals on those routes [R]. Kwoka & Shumilkina (JIE 2010), USAir–Piedmont: removing a *potential* entrant raised fares 5–6% [R]. Brueckner, Lee & Singer predicted United–Continental would raise the partners' fare revenue 0.08% and American–US Airways 0.17%; Carlton, Israel, MacSwain & Orlov (IJIO 2019) find only small mixed effects from the three big mergers [V] | Effect of consolidation on fares: large in a no-LCC world, about zero where LCCs have scale | as cited |
| Capacity discipline, 2008–15 | United's CFO in 2015: capacity grown "at less than GDP for eight consecutive years" [V]. James B. Stewart, "'Discipline' for Airlines, Pain for Fliers", NYT, 11 Jun 2015. DOJ demands to American, Delta, Southwest and United (over 80% of domestic seats) reported 1 Jul 2015. DOJ closed without action (Kendall & Carey, "Obama Antitrust Enforcers Won't Bring Action in Airline Probe", WSJ, 11 Jan 2017). Class action (MDL 2656, D.D.C.; alleged conspiracy Jan 2009 to mid-2015): **Southwest $15m, American $45m** (approved after a fairness hearing on 22 Mar 2019); **Delta's and United's summary-judgment motions denied 5 Sep 2023**. Seats about 2% lower when all legacies talk discipline | A "discipline" regime variable: legacies cap growth at or below GDP, and each one's adherence depends on the others' | [V]. Later Delta/United settlements, if any, not verified |
| Industry losses | Borenstein (2011): US carriers lost $59bn (2009 dollars) on domestic operations, 1979–2009: −$10bn in 1979–89, +$5bn in the 1990s, −$54bn in 2000–09 | Sanity check: in the long run the average carrier earns less than its cost of capital | [V] |
| Common ownership | Azar, Schmalz & Tecu (J. Finance 2018): fares 3–7% higher than under separate ownership (10–12% instrumented with BlackRock–BGI). Disputed by Dennis, Gerardi & Schenone and by DOJ economists | Optional; low priority | [V, R] |

---

## 6. Non-price competitive weapons

| Weapon | Documented facts | Model hook |
|---|---|---|
| Frequent-flyer programmes | AAdvantage launched 1 May 1981, seeded from Sabre: the 130,000 most frequent flyers and 60,000 Admirals Club members pre-enrolled. United's Mileage Plus followed in under a week. [V] United used mileage bonuses, including free trips, against Independence Air at Dulles. [V] | Loyalty stock per carrier per origin city, growing with local share. It cuts the price elasticity of high-fare demand toward the dominant carrier. A promotion (double or triple miles) is a cheap, targeted counter to an entrant |
| Travel-agent commission overrides | "Frequent booker" programmes rewarding agents for share. [V, Borenstein] Base commissions capped in 1995 and cut to zero in 2002 [R] | While agencies matter in the era simulated, the dominant carrier at an origin gets a share bonus beyond its schedule share |
| CRS display bias and the halo effect | Apollo went into agencies in 1976, then Sabre and others. Systems first favoured their owners' flights; US rules required neutral displays (CAB rule 1984 [R]; neutral "by 1987" [V, Wikipedia]). After the bias ban the "halo" remained: agents booked the host carrier disproportionately. Co-host deals bought smaller carriers better display [R]. By some accounts Lorenzo bought Eastern partly for its System One CRS [V] | Era switch. Before 1984: a strong display-share bonus to CRS owners. 1984–2000: a small halo. After: none; NDC and direct channels instead |
| Corporate discounts | Share-based percentage rebates off list, secret, under 10% of volume on most routes in the early 1990s. [V] At DFW, contracts committing "all or substantially all" of a firm's travel in exchange for "modest discounts". [V] | Unobserved price cuts for part of business demand; a hub carrier's lock-in |
| Slots | DFW slots cut about 40% in some hours after the 1981 strike; later allocated by lottery, trade and sale. [V] Slot holdings at LGA, DCA, JFK and ORD were held and "babysat" with small aircraft; the 2013 American–US Airways settlement forced divestiture of 104 DCA and 34 LGA slots and gates at five airports [R]. The Bundeskartellamt threatened Lufthansa with a fine for refusing Germania suitable slots (21 Mar 2002) [V] | At constrained airports, entry needs slots. Incumbents hold slots at low utilisation rather than release them. Regulators hand slots to entrants on mergers or abuse findings |
| Gates and majority-in-interest clauses | Northwest: 64 Detroit gates on long leases. Spirit paid $100,000 for one added flight and 25% higher landing fees. [V] Compass got the worst bays in a rival's terminal. [V] Qantas refused Tiger ground handling. [V] Exclusive-use leases and majority-in-interest vetoes over airport capital spending: FAA/OST task force 1999, GAO 1990 and 1996 [R] | A gate-access cost and delay for entrants at fortress hubs (months, and a unit-cost penalty) |
| Bracketing an entrant's departures | American matched Vanguard's fares on flights that "bracketed" its times, and matched Western Pacific "on the American flights leaving at the same time of day". [V] JetBlue's "timeband" language in 2020. [V] | A time-banded match: lowest fares only on departures within about 1–2 hours either side of the rival |
| Capacity dumping and overlays | Sections 4.1–4.3; Braniff's complaint about American in 1982. Qantas's 65% line | Rule 6 below |
| Cloning the entrant's niche | American's 56-seat Fokker 100s at Love Field within 26 days. [V] | A response where the entrant's edge is an airport or product, not cost |
| Regional partners and scope clauses | United's regional partner at Dulles became Independence Air; United replaced the feed. [V] Scope clauses cap the size and number of regional jets flown for a major [R] | Regional feed is contractual and can defect. Scope clauses limit how cheaply a legacy can add frequency |
| Airport incentive deals | Ryanair at Charleroi: Commission Decision 2004/393/EC (12 Feb 2004) found part of the Walloon and airport discounts to be unlawful aid, about €4m repayable; annulled by the Court of First Instance in Dec 2008 (T-196/04) [R] | Secondary airports pay LCCs to come. Model as a negative airport charge for a new base, with clawback risk |
| State aid | Air France recapitalisation (FF20bn, 1994); Alitalia's repeated rescues (bridge loans in 2008 and 2017, later ruled unlawful); Olympic; Sabena; the US carriers' 2015 claim of $42bn of Gulf subsidies [R] | The soft budget constraint in the flag-carrier archetype |
| Supplier and lender pressure | Operators warned McDonnell Douglas and GE against a Laker rescue. [V] | Rare; optional |

---

## 7. Short-run pricing behaviour: stylised facts

### 7.1 Response-time evidence

| Stimulus | Response | Lag | Source |
|---|---|---|---|
| System-wide increase filed Thursday night or Friday | Rivals match Saturday–Sunday; the initiator withdraws Sunday night or Monday if a major has not | 1–2 days to match; 2–3 days to roll back | Borenstein/Hirsch 1993; AP 2011 [V] |
| Failed increase | Retry at a smaller size | about 1 week (scenario 1); 1–3 weeks typical | [V]; ESTIMATE |
| Fare sale launched Monday night | Matched Tuesday morning; expired by Thursday or Friday | under 12 hours to match; a 3–4 day purchase window | McCartney 2011 via excerpts [V] |
| Tactical cut in a rival's key market | Match there, plus the same fare in the aggressor's key market; both withdrawn | 1 day each way (scenario 2); two weeks in the pre-1993 pattern | [V] |
| Northwest's family promotion, 26 May 1992 | American's 50% sale, then the industry | about 1 day | [R] |
| Southwest's $5 close-in cut, Apr 2016 | American and United match on overlap routes only; Delta does not | days | AP [V] |
| Entry by a small LCC with one daily flight | Nothing | 6 months, until its load factor hit 88% and a second flight appeared | *Spirit* [V] |
| Entry in a 89%-share hub market | $69 on all flights plus a widebody | same day | *Spirit* [V] |
| Announced LCC build-up at a hub | Schedule additions planned "within days", flown in 2–10 weeks | under 1 week to decide; 2–10 weeks to fly | *AMR* [V] |
| Entry at an alternative airport | Incumbent opens a cloned service there | 26 days | Legend [V] |
| Strong LCC gains both endpoints | Incumbents cut fares on that route only | starts about 3 quarters before entry | Goolsbee & Syverson [V] |
| LCC launch announced | Incumbent seat sale just before the first flight | −2 days | Jetstar and Tiger [V] |
| Entrant exits | Fares restored; capacity trimmed | fares within days to 2 months (+30% to +80%); capacity back to base in 2–6 months | *AMR*, *Spirit* [V] |

### 7.2 Depth, scope and form of matches

- **Depth.** Match to parity with the entrant's fare; do not undercut ("only matched, and never undercut"). Typical cuts against an LCC: −26% (Kansas City round 1), about −40% (Wichita), −35% to −45% (Colorado Springs), −60% (Lufthansa), −61% to −86% on the lowest fares (Northwest). [V]
- **Scope by time of day.** Stage 1: the entrant's time band only. Stage 2: all flights. Stage 3: all flights with yield-management overrides so the low fare is always open. [V, Colorado Springs sequence, 1995–96]
- **Scope by product.** The normal match is a restricted, capacity-controlled fare (Ultimate Super Saver; basic economy). The aggressive match removes restrictions (American against Sun Jet) or covers unrestricted fares too (Northwest's $49). [V]
- **Inventory.** In the two Northwest cases 40% and 75% of the incumbent's passengers travelled at or below the entrant's fare. That is the aggressive end. A routine match exposes perhaps 10–30% of seats. ESTIMATE
- **Nonstop against connecting.** Connecting carriers are the natural discounters in a hub carrier's nonstop markets: Northwest had under 60% of Detroit–Los Angeles traffic as the only nonstop. [V, Borenstein] Matching an entrant's *connecting* fare with a nonstop is an escalation reserved for exclusion (American against Vanguard). [V]
- **Geographic scope.** Cuts are confined to the threatened or overlapping routes. Not nearby airports; not the network. [V, Goolsbee & Syverson; AP 2016]
- **Fare sales.** Launch early in the week; 3-day purchase windows are common. [V] Typical depth 20–40% off the lowest regular fare, 14–21 day advance purchase, travel on off-peak days in a window 2–12 weeks out. ESTIMATE from general practice. The 1992 extreme was 50% off, a 10-day purchase window and travel across a whole summer. [R]
- **Fuel-driven increases.** $4–$10 round trip (about 1–3%) each; about 20 attempts in a fuel-spike year, half of them sticking, cumulates to +15–25% on base fares. ESTIMATE from the tallies
- **Sales offset hikes.** "Frequent sales can offset base fare increases." [V, AP 2016] The published base and the realised yield diverge; track both.

### 7.3 Price leadership and the asymmetry

- Downward: anyone can lead. The lowest-price-preferring firm sets the level (Gertner, via Borenstein). Legacies match each other nearly always. Legacies match an LCC or ULCC only where they overlap, and only once the entrant matters (share or load-factor trigger).
- Upward: the carrier with the largest presence leads. All significant carriers must follow within one weekend. Any LCC with meaningful overlap can veto, usually only in its own markets, which gives partial rollbacks.
- On a hub route the hub carrier wants the higher fare and connecting rivals want the lower. Cross-market threats are how the hub carrier gets its way. [V, CIS]

### 7.4 Exit

| Entrant / route | From incumbent's response to exit | Trough load factor | Source |
|---|---|---|---|
| Spirit, Detroit–Philadelphia | 3.3 months | 31% | [V] |
| Spirit, Detroit–Boston | 5.5 months | 17–31% throughout | [V] |
| Vanguard, DFW build-up, 1996 | 2–3 months | n/a | [V] |
| Vanguard, DFW–Kansas City, 1995 | cut to one flight after 3 months; exit after 11 | n/a | [V] |
| Sun Jet, DFW–Long Beach | 12 months | n/a | [V] |
| Western Pacific, DFW–Colorado Springs | 27 months after the first response; 13 after full escalation | 30% | [V] |
| Reno Air, Minneapolis | "within months" | n/a | [V] |
| Germania, Berlin–Frankfurt | abandoned its own €55 counter-cut in under 2 months | n/a | [V] |
| Legend (whole airline) | 7 months after American's clone | n/a | [V] |
| Independence Air (whole airline) | 18.5 months from launch; visibly failing at 4 | "far below capacity" | [V] |
| Laker | 4 months after Pan Am's 66% cut | loads halved | [V] |

**Spool-up.** Spirit's unopposed Detroit–Philadelphia flight took six months to go from 64% to 88.5% load factor. [V] Independence Air never got there. A 6–12 month maturation curve for a new route is consistent with this.

**Incumbents** review route profitability monthly on a trailing-twelve-month basis and act on persistent negatives. [V, American's FAUDNC process] They defer that discipline on routes where an entrant is present.

**Sub-brand responses** live 1.5–5 years (section 3.3).

---

## 8. Model recommendations

A rule set for an automated competitor. Every parameter has a default; "Ev." points to the evidence above. **Where a default is not followed by an "Ev." reference that contains that number, it is an ESTIMATE**: a starting value chosen to be consistent with sections 1–7, to be tuned against the calibration targets in 8.4.

### 8.1 Clock, information and state

- **Decision ticks.** Pricing: 4 per weekday and 1 per weekend day in ATPCO-style markets; hourly for international or modern markets (1.1). Capacity: schedule changes take effect 2–10 weeks after the decision (7.1); new stations and re-entries take 1–4 months.
- **Observable to all:** every rival's published fares by market and fare product after each tick; schedules and seats; which buckets are open.
- **Not observable:** seats sold per bucket; corporate discounts; cash.
- **Estimable at a cost:** a rival's load factor (gate counts), break-even load factor and cash runway. Give fortress-hub carriers this ability; give others a noisy version.
- **Per carrier-market state:** capacity share; frequency share; hub or focus status at each end; local-origin share at each end; trailing-12-month contribution; the rival's entry date, flights per day and estimated load factor; a "fight" flag with a stage (0–3); the pre-fight fare ladder (for restoration).
- **Per carrier-pair state:** number of overlapping markets, and for each side its top-N markets by revenue ("where it hurts").

### 8.2 Rules

**Rule 1: match a cut by a comparable carrier.**

- Trigger: a legacy or hybrid with at least `θ_share = 5%` of market seats files a lower fare in a product you sell.
- Action: match exactly at the next tick. Probability `p_match_peer = 0.95`.
- Never undercut the cutter. If you are the nonstop and it connects, match only if its connecting share exceeds `θ_connect = 15%` (ESTIMATE); otherwise ignore it.
- Ev.: 1.6, 2.3, Gertner; Delta's refusal in April 2016 shows the probability is below 1.

**Rule 2: match an LCC or ULCC.**

- Stage 0, ignore: while the entrant has no more than `n_ignore = 1` daily flight **and** an estimated load factor below `LF_trigger = 75%` **and** under `θ_share_lcc = 10%` of seats. Ev.: Detroit–Philadelphia, six months of no response until 88% and a second flight.
- Stage 1, banded match: match the entrant's lowest fare, with restrictions, on departures within `band = ±90 min` of the entrant's; expose `inv_1 = 15%` of seats. Ev.: Colorado Springs, July 1995; People Express.
- Stage 2, full match: all flights; `inv_2 = 40%` of seats. Ev.: Colorado Springs, Sept 1996; Detroit–Philadelphia, 40.5% at the entrant's fare.
- Stage 3, saturation: full match, unrestricted fares included, yield-management override; `inv_3 = 75%`. Ev.: Detroit–Boston, 74.5%; "ensure that lower fares were always available".
- Escalate one stage each `t_escalate = 3 months` if the entrant is still there and your archetype allows it. Go straight to the highest stage your archetype allows if the entrant announces at least `n_buildup = 3` new routes from your hub in one move (Vanguard 1996), or if your seat share exceeds `θ_fortress = 85%` (Detroit–Boston: same day).
- Product form: where the era has unbundled fares, do Stage 1–2 through a basic-economy class. Its effect on the entrant: −10% unit revenue in year 1, −20% by year 2 (Spirit).
- Scope: the overlapping route only (Goolsbee & Syverson; AP 2016).

**Rule 3: pre-empt a credible strong entrant.**

- Trigger: a low-cost carrier with a strong balance sheet establishes both endpoints of a route you serve. Entry hazard for that route multiplies by about 70.
- Action: cut average fares on that route only, along this path: −5% three quarters ahead, −10% two quarters, −17% at endpoint presence, −21% at actual entry, −25% to −29% in steady state. No capacity increase.
- If entry is already announced and certain, skip the pre-emption (the 223-route result) and go straight to accommodation.
- Ev.: Goolsbee & Syverson.

**Rule 4: follow an increase.**

- Initiation. Who: the carrier with the largest seat share across the affected markets, or any carrier whose fuel cost rose more than `x_fuel = 5%` in the past month. When: last tick Thursday or a Friday tick. Size: `Δ = 1–3%` of base fare ($4–$10 round trip). Coverage: all markets, less those where an LCC holds over 10% of seats if `carve_out = true`.
- Follow probability by archetype, within 48 hours: network legacy 0.85; hybrid 0.6; Southwest-type LCC 0.45; ULCC 0.10; state-supported flag 0.9. ESTIMATE, set so that system-wide success is about 50% in normal years (1.4).
- Raise the follow probabilities when industry load factor is high and fuel is rising. Calibration targets: success 70–75% in a 2007–08 fuel spike; 25–45% in 2011–14; attempts per year 20+ in spike years and 3–5 in slumps.
- Rollback: at the last weekend tick, in every market where a carrier with at least `θ_veto = 10%` of seats has not matched, withdraw. If more than `ρ = 30%` of the initiator's revenue base has been withdrawn, withdraw everywhere (ESTIMATE).
- Retry: after a failure, wait 1–3 weeks and retry at half the size (scenario 1). After two straight failures, wait for a cost or demand change.
- A matched increase can still be reversed weeks later by a spoiler (Southwest, April 2016). Keep `p_late_reversal = 0.05` per month for LCC archetypes.

**Rule 5: cross-market retaliation.**

- Trigger: a rival with a hub of its own cuts fares (Rule 1 conditions) in a market where you are dominant (your hub; local share over 50%), **or** refuses a Rule 4 increase there.
- Action in the same tick: (a) match in the attacked market; (b) file the same fare level in `k = 1–5` of the aggressor's most valuable markets where you already have service (usually connections over your hub), with a planned life of `d_punish = 1–14 days`. Five markets is JetBlue 2019; one is the DOJ scenario; two weeks is the pre-1993 practice.
- Withdraw (b) at the first tick after the aggressor withdraws. If it does not withdraw within `d_punish`, renew once, then treat it as a Rule 1 steady state.
- Aggressor's side: on seeing punishment linked to its cut, withdraw the cut with probability `p_fold = 0.7` (ESTIMATE; DOJ's 50-plus agreements imply folding was the norm). ULCCs and state-backed carriers fold less (0.2–0.3).
- Deterrence: the propensity to *initiate* a cut in a rival's dominant market falls with multimarket contact. Calibrate so equilibrium fares are 3–5% higher across the inter-quartile range of contact, with the effect saturating at high contact (Evans & Kessides; Ciliberto & Williams).
- Regime switch `signalling_allowed`: before the 1994 rules, allow pre-announced increases with a 2-week lead and iterative negotiation; this raises Rule 4 success. After, live fares only.

**Rule 6: capacity response to entry.**

- Against a weak entrant (cash runway under 12 months, or under 10 aircraft) on a route from your hub:
  - Add `+35% to +75%` seats within 1–3 months. Ev.: Wichita +35%; Kansas City 8 to 14 round trips; Colorado Springs 5 to 9; Boston +24% frequency plus a widebody.
  - Cancel planned seasonal cuts and pull planned additions forward (Boston; Phoenix).
  - Hold peak-season frequency through the off-season (Colorado Springs).
  - Re-enter routes you had abandoned if the entrant is building a base (Long Beach, Cincinnati). Accept losses on these: the complaint says the re-entry diverted American's own LAX traffic.
  - Optional overlay of the entrant's own base (Reno; the 1982 Braniff complaint). High regulatory risk.
- Against a strong LCC: do not add capacity. Price per Rule 3, protect the loyalty base (6), and trim capacity if contribution turns negative. Ev.: American against Southwest; Goolsbee & Syverson's capacity result.
- Capacity-share floor (option): hold group capacity share at or above `s* = 65%`, adding up to 2 seats for each rival seat. Abandon after `T_floor = 2–3 years`, or the first year both carriers lose money. Ev.: Qantas 2011–14.
- Clone: if the entrant's edge is a restricted airport or a niche product, open the same product at the same airport within 1–2 months (Legend).
- Legal envelope for the US-style regime: keep route revenue at or above route variable cost and never undercut; then there is no liability (4.1).

**Rule 7: restoration after exit.**

- When the entrant leaves or shrinks to one flight or fewer: restore the pre-entry fare ladder within `t_restore_price = 0–2 months`. Outcomes: +30% in 2 months (Long Beach), +50% by 6 months (Wichita), up to +80% (Kansas City). Northwest's lowest unrestricted fare returned to $271–$461.
- Reduce capacity to the pre-fight level over `t_restore_cap = 2–6 months` (Kansas City 14 to 11 to 10; Wichita −30%). The Tenth Circuit noted capacity generally stayed somewhat above pre-episode levels; use a residual of +0–10%.
- Reputation: each observed fight lowers other small entrants' estimated payoff from entering that hub. The intent ("deterred entry into DFW routes by other LCCs") is documented; the size is an ESTIMATE: −20% on perceived entry NPV per fight in the past three years.

**Rule 8: the entrant's decisions.**

- Route choice: prefer routes where both endpoints are already served (Berry; Goolsbee & Syverson) and where the incumbent's premium is high (hub local markets), unless a recent fight record deters (Rule 7).
- Spool-up: load factor ramps to maturity over 6–12 months if unopposed (64% to 88% in six months).
- Size: enter small. One or two daily flights sits under the incumbent's Stage 0 threshold (Gertner's capacity-constrained low-price equilibrium). Adding a second or third frequency, or several spokes at once, is what triggers Stage 2–3.
- Exit a route when its load factor has been under break-even for `m_exit` straight months: ULCC 2–3; small start-up 3–6; LCC with reserves 12–24; legacy on a trailing-12-month negative contribution, 12 months, or 18–36 during a fight; state-supported flag carrier: no automatic exit.
- A counter-cut below the incumbent's match does not hold: Germania's €55 lasted under two months.
- Airline failure: cash below `c_min` of 1–2 months of operating cost. Financial distress invites escalation by incumbents (Laker, 1981).

**Rule 9: fare sales.**

- Launch Monday night or Tuesday; rivals with overlap match by the next tick; the purchase window is 3 days; depth 20–40%; 14–21 day advance purchase; off-peak travel 2–12 weeks out; 10–20% of seats on eligible flights. (Timing is [V]; depth, window and inventory are ESTIMATE.)
- Frequency: more sales when forward bookings run behind plan. A sale by the largest carrier is matched market-wide; a sale by a ULCC is matched only on overlap routes.
- Tail risk: with `p_runaway = 2% a year` in distressed periods, a promotional match escalates to a system-wide deep sale (50%, 10-day window) that every carrier matches within a day and that destroys a season's yield (1992).

**Rule 10: structural responses.**

- Airline within an airline: a legacy that has lost more than `θ_loss = 15–20%` of local share in a region to LCCs may launch a sub-brand. Outcomes: CASM only 5–15% below the parent if labour is shared (ESTIMATE); life 1.5–5 years; closed at the next cash crisis. A subsidiary with a separate certificate and labour (Jetstar, Level) can last.
- Unbundled fare class: far cheaper than a sub-brand and more effective (3.4).
- Loyalty counter-attack: targeted mileage bonuses in the entrant's markets (Dulles 2004).
- Access denial: at gate- or slot-constrained hubs the entrant waits 3–12 months and pays a unit-cost penalty (25% higher landing fees at Detroit). The regulator archetype decides whether slots or gates are handed over.
- Buy the rival: Ryanair/Buzz (about €20m), easyJet/Go (£374m), Virgin Australia/Tiger (A$35m for 60%, then A$1), Texas Air/People Express (about $125m). A fare war often ends in acquisition at a distressed price.

### 8.3 Carrier archetypes and reaction functions

| Parameter | Network legacy at a fortress hub | Legacy off-hub / in a rival's hub | LCC (Southwest-type) | ULCC | Flag carrier with state support | Start-up entrant |
|---|---|---|---|---|---|---|
| Goal | Protect the hub premium and connectivity; deter entry | Fill connecting seats; avoid provoking | Stimulate and grow from a cost advantage (20% or more below the nearest rival) | Lowest base fare; ancillaries; opportunistic | Share, connectivity, national presence | Survive spool-up |
| Match a peer's cut (R1) | 0.95, next tick | 0.95 | 0.9 | 0.5 (often already lower) | 0.95 | 0.8 |
| Response to a small LCC (R2) | Stage 0, then 1 to 3; escalates every 3 months; straight to 3 on a hub build-up | Stage 0–1 only | Match if overlapping; no escalation | Hold own price | Stage 1–2; slower (6-month steps) | n/a |
| Pre-empt a strong entrant (R3) | Yes, the full path | Partial (half the depth) | n/a | n/a | Weak or late (starts at entry) | n/a |
| Lead increases (R4) | Yes, in its hub markets | Sometimes | Occasionally | Never | Yes | Never |
| Follow increases (R4) | 0.85 | 0.85 | 0.45; the main spoiler | 0.10 | 0.9 | 0.3 |
| Cross-market retaliation (R5) | Yes, k = 3–5 | Target, rarely initiator | No | No | Rarely; bilateral politics instead | No |
| Folds when punished | 0.7 | 0.8 | 0.3 | 0.2 | 0.3 | 0.5 |
| Capacity against a weak entrant (R6) | +35–75%; re-entry; possible overlay | none | none | none | +10–30%, held for years | n/a |
| Capacity against a strong LCC | none; trim | trim or exit | n/a | exit overlap if matched | hold | n/a |
| Share floor | optional (65% type) | none | none | none | often, explicit or political | none |
| Exit patience (R8) | 18–36 months in a fight; 12 normally | 6–12 months | 12–24 | 2–3 | none; needs a restructuring event | 3–6, limited by cash |
| Non-price weapons | FFP, corporate deals, gates and slots, agency overrides, bracketing | FFP | low cost, simple product, secondary airports | airport incentives, ancillaries | slots, bilaterals, state aid, home-market FFP | airport incentives |
| Typical failure | Legal or agency action; de-hubs after years of LCC share loss | Withdrawal | Rare: cost creep, saturation | Yield collapse once legacies match (−20% in 2 years) | Periodic insolvency and rescue | Runs out of cash in 3–18 months once fought |

All probabilities and durations in this table are ESTIMATES. The orderings between archetypes are what the evidence supports; the levels are not measured anywhere.

**Reaction function in one line each.**

- **Fortress hub:** `response = f(entrant strength, entrant load factor, entrant growth, own share)`. Weak and growing: fight in stages, restore afterwards. Strong: price down early, hold capacity, compete on loyalty.
- **Off-hub legacy:** discount by connection in others' hubs up to the point of retaliation, then fold.
- **LCC:** price off own cost; ignore signals; veto increases where they do not suit; enter where both endpoints exist.
- **ULCC:** undercut everyone on base fare; stay under the Stage 0 threshold on each route; churn routes quickly.
- **State-supported flag:** match price, hold capacity, do not exit; resolution comes through rescue, merger or a regulator.
- **Start-up:** enter one or two flights at a time at 40–70% below the incumbent's restricted fare; a second frequency is the risky step.

### 8.4 Calibration targets

A route-fare model with these agents should reproduce:

1. Southwest-type nonstop presence: −27% to −29% on average fares (−46% in a pre-2000 world). Other LCC: −13% to −18%. Adjacent-airport LCC: −5% to −17%. Potential entry: −4% to −10%.
2. Loss of a legacy nonstop competitor: 0 to +4% where LCCs have scale; +8% to +12% where they do not.
3. Hub premium for the dominant carrier's local traffic: 20–27% around 1990, declining by more than half over 20 years; concentrated at the top of the fare ladder.
4. Multimarket contact: +3–5% across the inter-quartile range.
5. Fare dispersion: Gini about 0.22; an LCC's entry cuts the 90th percentile about twice as much as the 10th on business routes.
6. System-wide increases: about half stick; 3–23 attempts a year depending on fuel and demand.
7. Incumbent fares fall 17% before a strong LCC's first flight; entrant traffic stimulation of +28% (2010s) to +200–300% (early 1990s).
8. A fought small entrant leaves in 3–12 months; fares are restored +30–80% within 0–6 months.
9. ULCC unit revenue: −10% to −20% over two years once legacies match with unbundled fares.
10. Capacity discipline: when every legacy in a market is "disciplined", seats are about 2% lower.
11. Over a long horizon the average carrier does not earn its cost of capital (−$59bn on US domestic operations, 1979–2009), with one or two lethal fare wars per decade.

### 8.5 Regulator regimes (game rules that change the reaction functions)

| Regime | Rule | Effect on agents |
|---|---|---|
| US after *Brooke Group* / *AMR* | Liability only if price is below average variable cost on the route **and** recoupment is likely. Matching is safe | Fortress-hub Stage 3 and Rule 6 are available |
| US agency pressure (1993; 1998 proposal) | Investigation if passengers carried at the matched fare exceed the entrant's capacity or low-fare traffic; hub overlays draw intervention | Caps inventory at the matched fare near the entrant's seat count; overlays forbidden |
| German 2002 type | Incumbent must keep a price gap equal to its quality premium (about 35% of the entrant's fare, capped) for two years; must surrender suitable slots | Stage 2–3 forbidden; entrant gets a two-year protected spool-up |
| Canadian 2003 type | Avoidable-cost test including overhead on added capacity | Rule 6 capacity additions forbidden unless they cover avoidable cost |
| Pre-1994 signalling | Future-dated fares and footnote links allowed | Higher success for increases; explicit two-week punish-and-withdraw trades |
| State-aid regime | Rescue aid allowed, sometimes with capacity or slot remedies | Enables the flag archetype; periodic forced shrinkage |

### 8.6 Known gaps and items to verify

- [R] items to check before external use: the 1992 detail (fare tiers and cut sizes, 26–27 May sequence, autumn abandonment, jury verdict, $4.8bn); Kim & Singal's 9.44% and 12.17%; Kwoka & Shumilkina's 5–6%; GAO's 27%; DOT's 41% and 54%; the DOT 1998 three-trigger wording; Southwest's BWI and PHL dates and the Siegel quote; American's and United's 2015–18 statements and share-price moves; Qantas and Virgin FY2014 losses; SpiceJet's 2014 flash sales; the Charleroi amounts; transatlantic basic-economy dates and the IAG stake; HBS case numbers.
- Not found: a clean source for typical fare-sale depth and purchase window; when the 16:00 ATPCO feed was added; the final 2013 hike tally; whether Delta and United later settled the capacity class action; the eventual outcome of *Spirit v. Northwest* on remand.
- Not researched in depth: south-east and north-east Asian LCC wars; Latin America; the Gulf carriers' effect on European and Asian long-haul yields; China's regulated fare bands.
- Teaching cases to pull, case numbers from memory [R]: HBS "People Express Airlines: Rise and Decline" (490-012); "American Airlines' Value Pricing (A)" (594-001); "Southwest Airlines 1993 (A)" (694-023); "Dogfight over Europe: Ryanair (A)-(C)" (700-115 onward); "JetBlue Airways: Starting from Scratch" (801-354); "Delta Air Lines (A): The Low-Cost Carrier Threat" and "(B): The Launch of Song" (704-403, 704-439). Also Petzinger, *Hard Landing* (1995), and Cross, *Revenue Management* (1997), for the Crandall, Burr and Laker narratives.

---

## 9. Sources

Read directly this session:

- Borenstein, "Rapid Communication and Price Fixing: The Airline Tariff Publishing Company Case" (1997): https://faculty.haas.berkeley.edu/borenste/download/atpcase1.pdf
- DOJ Competitive Impact Statement, *US v. Airline Tariff Publishing Co.*: https://www.govinfo.gov/content/pkg/FR-1994-03-31/html/94-7672.htm
- DOJ complaint, *US v. AMR Corp.*: https://www.justice.gov/atr/case-document/file/485581/dl
- *US v. AMR Corp.*, 335 F.3d 1109 (10th Cir. 2003): https://law.resource.org/pub/us/case/reporter/F3/335/335.F3d.1109.01-3202.html
- *Spirit Airlines v. Northwest Airlines*, 431 F.3d 917 (6th Cir. 2005): https://caselaw.findlaw.com/us-6th-circuit/1060723.html
- DOJ complaint, *US v. American Airlines and Crandall* (1983): https://www.appliedantitrust.com/06_conspiracy/d_invitations/american1983/american_airlines_ndtex_complaint2_23_1983.pdf
- *Continental Airlines v. American Airlines*, 824 F. Supp. 689 (S.D. Tex. 1993), opening only: https://leagle.com/decision/19931513824fsupp68911413
- *In re Domestic Airline Travel Antitrust Litigation*, memorandum opinion, 5 Sep 2023: https://static.bfalaw.com/docs/MemoOpinion.pdf
- Goolsbee & Syverson, NBER w11072: https://www.nber.org/system/files/working_papers/w11072/w11072.pdf
- Brueckner, Lee & Singer, IATA briefing, May 2010: https://www.iata.org/en/iata-repository/publications/economic-reports/network-vs.-lcc-competition-and-airfares-new-us-empirical-evidence
- Brueckner, airline industry overview: https://sites.socsci.uci.edu/~jkbrueck/Airline%20overview.pdf
- Ciliberto & Williams, working paper: https://mpra.ub.uni-muenchen.de/24888/1/MPRA_paper_24888.pdf
- Gerardi & Shapiro, Boston Fed WP 07-7: https://www.bostonfed.org/-/media/Documents/Workingpapers/PDF/wp0707.pdf
- Borenstein, NBER w16744: https://www.nber.org/system/files/working_papers/w16744/w16744.pdf
- Aryal, Ciliberto & Leyden: https://arxiv.org/abs/2102.05739
- Bundeskartellamt, 19 Feb 2002: https://www.bundeskartellamt.de/SharedDocs/Meldung/EN/Pressemitteilungen/2002/19_02_2002_Lufthansa_Germania_eng.html (also the 22 Jan and 10 Apr 2002 releases)
- AP (Koenig), 28 Apr 2016: https://www.ksl.com/article/39548223/southwest-delta-tangle-over-whether-to-cut-or-raise-fares
- NBC News, 26 Mar 2012: https://www.nbcnews.com/business/travel/airlines-successfully-hike-airfares-third-time-year-flna556085
- PBS NewsHour, 3 Jul 2015: https://www.pbs.org/newshour/nation/us-airline-probe-need-proof-beyond-business-usual
- View from the Wing, "How Airlines Collude To Raise Prices, Right In Front Of Regulators": https://viewfromthewing.com/?p=183949
- SBS, 5 Mar 2014: https://www.sbs.com.au/news/article/market-share-war-will-continue-says-joyce/2nrsylahb
- Travel Weekly, 4 May 2012: https://www.travelweekly.com.au/article/Qantas-vows-to-protect-market-share/
- CAPA, 21 Mar 2014: https://centreforaviation.com/analysis/reports/airline-overcapacity-is-concentrated-in-western-australia-can-qantas-and-virgin-recalibrate-158525
- Wikipedia, with the citations noted in the text: Laker Airways; People Express Airlines (1980s); Independence Air; Legend Airlines; Continental Lite; Song (airline); Ted (airline); MetroJet; Go Fly; Buzz (British airline); Bmibaby; Ryanair; Norwegian Long Haul; Level (airline); Wow Air; Virgin America; Wright Amendment; Southwest Effect; Spirit Airlines; Kingfisher Airlines; SpiceJet; Air Deccan; Tigerair Australia; Compass Airlines (Australia); AAdvantage; Computer reservation system; JetBlue; Vanguard Airlines; Reno Air.

Seen only as search-result excerpts (search budget exhausted before they could be opened, or the site refused the fetch):

- FareCompare hike-tally posts, 2011–15: https://www.farecompare.com/news/first-airfare-hike-of-2012/ and the related series
- Seaney on Fox News (2014 tally): https://www.foxnews.com/travel/as-airlines-hike-fares-they-ask-how-much-more-will-you-pay.print
- AP, 28 Mar 2011: https://m.washingtontimes.com/news/2011/mar/28/budget-airlines-balk-at-fare-hikes/
- AviationPros, "Biggest U.S. carriers rescind $10 fare hike": https://www.aviationpros.com/home/news/10388862/biggest-us-carriers-rescind-10-fare-hike
- TravelMole, "US airlines' wings clipped for $6": https://travelmole.com/news/us-airlines-wings-clipped-for-6
- Kralev on ATPCO feed times: https://nicholaskralev.com/tag/atpco ; FlyerTalk: https://www.flyertalk.com/forum/28920827-post6.html
- Secondary coverage of McCartney's Jan 2011 column: https://consumerist.com/2011/02/01/tuesday-and-wednesday-are-best-days-to-buy-airfare
- CS Monitor, 14 Apr and 2 Jun 1992: https://csmonitor.com/1992/0414/14071.html , https://csmonitor.com/1992/0602/02071.html ; Bloomberg, "The Airline Mess", 5 Jul 1992
- DOJ speech, "Predation in the Airline Industry": https://www.justice.gov/atr/speech/predation-airline-industry ; Las Vegas Sun, 15 Apr 1999; MPR, 8 Apr 1998
- Competition Tribunal, *Commissioner v. Air Canada*: https://decisions.ct-tc.gc.ca/ct-tc/cdo/en/464434/1/document.do
- Morrison (2001): https://ideas.repec.org/a/tpe/jtecpo/v35y2001i2p239-256.html
- Kim & Singal (1993): https://ideas.repec.org/a/aea/aecrev/v83y1993i3p549-69.html
- Miller (2010): https://ideas.repec.org/a/ucp/jlawec/doi10.1086-605294.html
- Wei & Hansen (2005): https://ideas.repec.org/a/eee/transe/v41y2005i4p315-327.html
- Azar, Schmalz & Tecu (2018): https://papers.ssrn.com/abstract=2427345
- NZ Herald on Virgin and Qantas domestic earnings: https://nzherald.co.nz/business/absorbing-overcapacity-will-take-time-virgin-boss/IBJJUUVKNF5K7QAYABG6QO4CCQ
- PhocusWire and Fortune on the Southwest ($15m) and American ($45m) settlements: https://www.phocuswire.com/Southwest-agrees-to-settle-antitrust-lawsuit-over-capacity-discipline , https://fortune.com/2018/06/16/american-airlines-lawsuit/
