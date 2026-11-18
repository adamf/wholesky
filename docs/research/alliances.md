# Airline alliances and partnerships: contract structures, money flows, system behaviour

Research notes for modelling Star Alliance, oneworld, SkyTeam, bilateral codeshares, immunised joint ventures, interline and regional feed in a reservation gateway and a world airline simulator. Compiled 18 November 2026.

## How to read this document

Every factual claim carries one of three provenance marks.

| Mark | Meaning |
|---|---|
| **[V]** | Verified in this research session against the cited source (source list at the end; short source tag follows the mark). |
| **[R]** | Recalled from general industry knowledge. Not re-verified in this session. Treat as a lead to confirm before hard-coding. |
| **ESTIMATE** | My own modelling estimate or a figure I derived by arithmetic from verified inputs. Not a published number. |

Limits of this research, stated once so they are not missed:

- The session's shared web-search budget ran out part-way through. About 30 searches ran; everything after that came from fetching known primary URLs (IATA, DOJ, EC, SEC, GAO, eCFR mirror, Wikipedia, trade press). Topics that depended on further searching are marked [R] or ESTIMATE rather than filled in.
- Not obtained: alliance entrance fees and annual dues, Star Alliance Services GmbH staff and budget, inter-airline FFP rates per mile, lounge per-visit charges, codeshare commission percentages, the text of DOT orders (transportation.gov and regulations.gov blocked automated fetches), the HBS teaching cases, and airline annual-report statements of revenue attributable to alliances. Where a default is still needed for the model, it is given as an ESTIMATE.
- Five figures in the brief did not check out as stated. See "Corrections to the brief" immediately below.

### Corrections to the brief

| Claim in the brief | What the sources say |
|---|---|
| Brueckner & Whalen (2000): alliance partners charge "18-28%" less on interline | The published abstract says "approximately 25 percent". Whalen's 1999 dissertation reports 20-28%. I found no source for an 18% lower bound. [V: UCI ITS abstract; search summary of the dissertation] |
| Brueckner (2003): codeshare -8-17%, ATI -13-21%, "combined ~27%" | The two ranges are right for the REStat paper, but its own combined range is 17-30%. The 27% is the sum 7% (codeshare) + 4% (alliance) + 16% (ATI) from Brueckner's companion paper, as restated in Brueckner & Singer (2019). [V] |
| "~55-60% aligned" share of world traffic | True by RPK in 2012-2018 (59% in 2012, 62.5% in 2014, 56% in 2018). Now out of date: 47.7% of RPKs in 2020, and about 43-45% of seats in 2022-2024. Unaligned carriers are the majority. [V: IATA WATS releases; OAG] |
| Northwest-KLM "ATI Jan 1993" | GAO (1995) says DOT granted immunity in November 1992, tied to the September 1992 open-skies agreement. The final order is usually dated January 1993 [R]. Use Nov 1992 (grant) / Jan 1993 (final) and 1993 as first year of operation. [V: GAO] |
| Delta-Aeromexico as a live JV | DOT ordered the immunity terminated on 15 Sept 2025, effective 1 Jan 2026. The Eleventh Circuit stayed that on 12 Nov 2025 and vacated it on 20 Aug 2026. The JV continues. [V: trade press and a law-firm note; the vacatur rests on two secondary sources] |

---

## 1. The ladder of cooperation

### Summary: what each rung changes

| Rung | Contract | Who prices | Who holds inventory | Revenue split | Settlement | Regulatory gate |
|---|---|---|---|---|---|---|
| (a) Interline | MITA or bilateral interline agreement, plus bilateral e-ticketing agreement | Each carrier files its own fares; through fares are constructed or published jointly | Each carrier, own flights, own code | Prorated per coupon: MPA by default, SPA if agreed. Ticketing carrier keeps an interline service charge | Monthly interline billing through the IATA Clearing House | None beyond normal tariff rules |
| (b) Codeshare | Bilateral codeshare agreement, usually with its own SPA | Marketing carrier prices seats sold under its code. No joint pricing without immunity | Operating carrier (free sale) or marketing carrier for its block (block space) | Operating carrier receives the coupon value under the codeshare SPA less a commission, or a fixed seat price for a block | Same interline billing channel | Route authority for the marketing carrier; DOT codeshare approval and disclosure in the US |
| (c) Alliance membership | Multilateral alliance agreement plus a full mesh of bilaterals | Unchanged: each carrier | Unchanged | Unchanged, plus FFP accrual and redemption payments and lounge charges between members | FFP and lounge invoices, usually through the same clearing house | None by itself. No immunity |
| (d) Immunised JV | Joint venture agreement over a defined market, with antitrust immunity or equivalent clearance | Jointly | Each carrier still holds its own flights, but availability is steered to the joint optimum | Pooled route-group revenue (or revenue less agreed costs), shared by formula regardless of who flew the passenger | Periodic JV true-up payments on top of normal coupon billing | DOT immunity under 49 U.S.C. 41308-41309, EC Article 101 commitments, ACCC authorisation, and so on |
| (e) Equity | Share purchase, sometimes with board seats | Unchanged unless combined with (d) | Unchanged | Dividends and equity-accounted results | n/a | Foreign-ownership caps |

The cells above summarise the detail in 1(a) to 1(e), where each claim carries its mark. Sources for the ladder as a framework: the EC/DOT 2010 joint report's "spectrum of airline co-operation" as reproduced by IATA Economics (2011) [V]; AAI (2018), which lists interline, FFP sharing, codeshare, price and schedule coordination, and "full metal" revenue and profit sharing in that order [V].

### 1(a) Interline e-ticketing and baggage

**Legal frame.**
- MITA (IATA Multilateral Interline Traffic Agreements) is the standard passenger and cargo interline agreement. It sets the basic rules for collecting money and issuing documents for carriage on each other's services, plus liability and procedures. IATA Resolution 780 is the framework agreement for multilateral interlining: provision of fares and schedules, acceptance of passengers and baggage, interline billing, baggage handling and claims. It assumes electronic tickets. [V: IATA MITA page; IATA IROPS guidance 2020]
- Airlines outside MITA, or wanting different terms, use bilateral interline agreements. E-ticketing between two carriers needs its own bilateral: IATA's BIETA database lists 4,626 bilateral interline e-ticketing agreements. The MITA manual was in its 108th edition, with more than 220 new agreements and 30 withdrawals in that edition. [V: IATA MITA page]
- Scale: over 8% of passenger segments flown by IATA member airlines were sold by another airline; interline revenue is estimated at over USD 52 billion a year; about 10% of itineraries involve interline (IATA Direct Data Solutions, 2018 data). [V: IATA "Future of Interline"]
- The three customer promises IATA attaches to interline: one price and one payment in one currency; seamless connections for passengers and bags; carriage to final destination after a disruption. [V: same]

**Settlement.**
- The IATA Clearing House handles over USD 60 billion in annual billings for more than 560 participants (about 330 airlines and 230 suppliers), with weekly claim filing, settlement in 8 currencies, and a stated 99.999% settlement success rate over three years. It provides set-off and default protection. [V: IATA ICH page]
- Mechanics [R]: the carrier that flew the coupon bills the ticketing (validating) carrier; invoices are submitted electronically through SIS; ICH nets all bilateral positions and each member settles one net amount per clearance period. US carriers clear through the Airlines Clearing House, which interoperates with ICH [V that ACH exists and partners on involuntary-reroute settlement: IATA IROPS guidance].
- ICH can suspend a member in default. That is the event that turns an airline failure into losses for its interline partners [R].

**Proration under the Multilateral Prorate Agreement (MPA-P).**
- The MPA-P is "a default agreement that describes processes for the proration of interline through-fares", administered by IATA's Prorate Agency. Participation is optional. The Prorate Agency publishes prorate factor miles, base amounts and airline-filed provisos in the Prorate Manual Passenger (ePMP). [V: IATA "Future of Interline"; IATA prorate page; search summary of IATA training pages]
- Method [R, standard practice]: each fare component is divided among the sectors it covers in proportion to each sector's prorate factor. Factors are distance-based with a taper and an area cost weighting, so a short sector gets more per mile than a long one. This is straight-rate proration (SRP).
- Provisos [R]: a carrier may file a requirement for specific sectors that overrides SRP, typically a fixed amount, a percentage of its local sector fare, or SRP subject to a minimum. Domestic add-on sectors are the common case. Base amounts are the reference local fares used by provisos and by the rule that no carrier receives more than its own local fare for the sector.
- Involuntary reroute uses two proration passes: first prorate the original ticket with provisos to find what the original carrier would have earned, then spread that value over the reissued coupons by SRP only. [V: IATA IROPS guidance]

Worked SRP example (illustrative factors, ESTIMATE, not the published ones):

| Sector | Carrier | Illustrative prorate factor | Share | Amount from a 1,000 through fare |
|---|---|---|---|---|
| Regional feeder, 300 nm | A | 900 | 900/4,400 = 20.5% | 204.55 |
| Long-haul trunk, 3,500 nm | B | 3,500 | 3,500/4,400 = 79.5% | 795.45 |

If A is the ticketing carrier and the default 9% interline service charge applies, B bills A 795.45 less 71.59 = 723.86.

**Special Prorate Agreements (SPAs).**
- Bilateral agreements that replace the MPA for defined traffic. Airlines "negotiate special prorate agreements to control revenue" on interline itineraries. [V: IATA "Future of Interline"]
- Usual forms [R]: a fixed amount per sector by booking class and season; a percentage of the through fare; a percentage of the carrier's local fare (a "net" local fare); or SRP with a floor and a ceiling. Scope is defined by origin and destination, point of sale, fare basis or RBD, and date range.
- Regulators use SPAs as a remedy: the EC required BA/AA/Iberia and Air France-KLM/Alitalia/Delta to offer SPAs and fare combinability to competitors on the routes of concern, so that rivals could reach the parties' connecting traffic. [V: EC IP/10/936, IP/15/4966]
- Before metal-neutral JVs, most immunised alliances shared revenue only as far as an SPA dictated. Northwest/KLM was the exception. [V: DOJ comments on Star, 2009]

**Interline service charge (ISC).**
- A percentage deducted from the amount owed to the carrying airline and kept by the ticketing airline, to pay for selling, ticketing, distribution and card costs. The one stated rate I found: 9% of the prorate value as the multilateral default, 5% or 7% commonly under SPAs, and 0% inside joint ventures. This comes from a vendor glossary, not from an IATA document, so treat the 9% as [V, secondary source only]. The MITA manual also contains separate multilateral ISC agreements for the United States [V: IATA MITA page], so the US rate may differ.

**Baggage and check-in at this rung** [R]: baggage is through-tagged under the interline agreement; the baggage allowance on a mixed itinerary follows IATA Resolution 302 (most significant carrier) outside the US and the first marketing carrier's rules under DOT regulation; through check-in needs a separate bilateral IATCI link between the two departure control systems. Without IATCI the passenger gets a bag tag to destination but must collect the onward boarding pass at the transfer point.

### 1(b) Codeshare

**Definition and types.**
- A marketing carrier files a schedule for a flight operated by another airline as if it were its own, and manages inventory for it in one of two ways: a block of seats agreed with the operating carrier, or, more often, free sale with automated access to the operating carrier's own inventory. Fares, ticketing and frequent-flyer entitlements are set by the marketing carrier as on its own flights. [V: IATA "Future of Interline"]
- "Block space codeshare was once very common but is less so now." [V: same]
- In 2017, 25% of all published flights were marketing codeshare flights, and each operating flight of an IATA member carried on average 2.4 marketing flight numbers. Filing of codeshare schedules is rising while passengers booked under codeshare flight numbers are stable. [V: same]
- Why airlines codeshare, in IATA's words: a codeshare connection gets priority in a GDS neutral display over the same connection sold as interline; the marketing carrier controls itinerary combination and pricing because GDS fare selection keys on the airline code; it marks a preferred tier of partner; and it permits a separate inventory allocation and different billing. The cost is complex, failure-prone system interaction. [V: same]

| Type | Inventory | Risk | Typical settlement | Notes |
|---|---|---|---|---|
| Free sale (free flow) | Operating carrier holds all seats. Marketing carrier sees availability by AVS status messages or interactive polling and sells into it in real time | Operating carrier | Per coupon flown, under the codeshare SPA, less commission | The modern default [V: IATA; OAG 2024] |
| Capped free sale | As free sale with a maximum number of seats per flight or class for the marketing carrier | Operating carrier | As above | [V: Wikipedia, unsourced section] |
| Soft block | Marketing carrier holds an allocation in its own system and may return unsold seats by a release date | Marketing carrier until release, operating carrier after | Seat price times seats kept or sold | [V: DWU Consulting] |
| Hard block | Marketing carrier buys a fixed number of seats at a fixed price and sets its own booking classes within the block. Seats are removed from the operating carrier's inventory | Marketing carrier | Fixed seat price times block, sold or not. Can be zero under a symmetric seat swap (for example 20 seats each way on parallel routes) | [V: Wikipedia; DWU Consulting] |

**Booking-class mapping and inventory control.**
- The two carriers agree a table mapping the marketing carrier's RBDs to the operating carrier's. In a JV the partners go further and align base fares and booking classes across all airlines. [V: Lufthansa Group Atlantic JV presentation, "Aligned base fares and booking classes across all airlines"]
- DOJ's description of how allied carriers handle yield management is the most useful public statement of system behaviour: partners "agree on how to map fares into each other's yield management systems"; each "treats fares partly booked on the other's metal as its own". DOJ noted this lets even non-immunised partners overcome double marginalisation. Without such mapping a carrier's revenue management would rank a shared-revenue alliance passenger below a wholly-owned-revenue local passenger. [V: DOJ comments on oneworld, Dec 2009]
- Message flows [R]: availability by AVS (open/closed by class) or by seamless polling of the operating carrier; sell by interactive Type A message with immediate confirmation, or by teletype with the risk of a later "unable"; the operating carrier holds a companion PNR; for a block, the marketing carrier sends passenger name lists (PNL/ADL) before departure.

**Revenue division.**
- The operating carrier generally keeps most of the revenue [V: OAG 2024; no percentage given]. The marketing carrier validates the ticket, the coupon shows the marketing flight number, and the operating carrier bills the coupon to the marketing carrier through interline billing at the value set by the codeshare SPA, less the agreed commission.
- No public source gave a typical commission. Bracketing from the ISC figures above: 9% multilateral default, 5-7% under SPAs, 0% inside JVs. ESTIMATE for a free-sale codeshare between unrelated carriers: 5-10% of the operating carrier's prorated coupon value. ESTIMATE for a hard block seat price: close to the operating carrier's expected net yield for the cabin at the block's implied load factor, roughly 75-85% of the average sold fare.
- GAO's 1995 data points on one-way codeshares: British Airways put its code on USAir domestic flights and kept most of the revenue, estimating USD 100 million of added revenue for April 1994 to March 1995 (USD 45 million codeshare, USD 55 million interline and other), against about USD 20 million for USAir. [V: GAO RCED-95-99]

**Schedule filing, flight numbers, display, disclosure.**
- SSIM data element identifiers: DEI 010 on the operating leg lists all commercial duplicate designators and flight numbers; DEI 050 on the marketing flight names the operating airline and flight. [V: OAG 2024]
- Minimum connecting time: for codeshare flights the MCT is established by the operating carrier unless the marketing carrier files its own; the marketing flight is resolved through its DEI 050. [V: IATA MCT Technical Guide v2.2]
- Marketing flight numbers conventionally sit in four-digit ranges reserved per partner, for example BA173 sold as AY5473, or American-operated US domestic flights sold as IB4945. [V: OAG 2024]. A common layout is 1-2999 own mainline, 3000-6999 regional affiliates, 7000-9999 other airlines' flights, but each carrier differs [R].
- Display bias: GAO found in 1995 that codeshare listings filled much of the first CRS screen in nearly 20% of cases reviewed, when travel agents made about 80% of bookings and booked from the first screen up to 90% of the time. The EU limited a codeshare flight to two listings in 1993. US rules set no limit. [V: GAO RCED-95-99]
- US disclosure rule, 14 CFR Part 257, "Disclosure of code-sharing arrangements and long-term wet leases" (64 FR 12851, 15 Mar 1999; amended 81 FR 76828, 3 Nov 2016, and 84 FR 15932, 16 Apr 2019) [V: Cornell LII]:
  - A code-sharing arrangement is one in which one carrier's designator code identifies a flight operated by another carrier. A long-term wet lease is aircraft plus crew dedicated to particular routes for more than 60 days.
  - Schedules and itinerary search results, on desktop and mobile, must show the operating carrier's corporate name and any other name it is held out under, in text no smaller than the itinerary font, adjacent to the flight. Roll-over, pop-up and linked disclosures do not comply. Static schedules need an asterisk or similar mark.
  - Oral notice the first time the flight is offered. Written notice at purchase reading "Operated by" followed by the corporate name and trade name, for example "operated by Jane Doe Airlines d/b/a QRS Express".
  - Print and web advertising must name all potential operating carriers. Broadcast advertising needs at least "Some flights are operated by other airlines."
- History: the first codeshare is usually dated to 1967 (Henson's Hagerstown Commuter with Allegheny). Qantas and American coined the term in 1989 and flew their first codeshares in 1990. [V: Wikipedia]. DOT-approved US/foreign codeshare alliances rose from 19 in 1992 to 61 at the end of 1994: 3 strategic, 8 regional, 50 point-specific, and about a third of the point-specific ones were later terminated. [V: GAO RCED-95-99]

### 1(c) Alliance membership

**What the alliance agreement adds.** No pricing or capacity coordination, and no immunity. Membership is a bundle of obligations layered over bilaterals:

| Obligation | Detail | Provenance |
|---|---|---|
| Bilateral mesh | A full member must have commercial relationships with every other member. A Star Connecting Partner needs them with only three | [V, secondary: summary of NerdWallet/PointsCrowd pages] |
| Reciprocal FFP | Earn and burn on all members, recognition of the alliance tiers | [V: Wikipedia Star; Lufthansa AJV deck] |
| Lounge access | Top tier and premium-cabin passengers admitted to members' lounges | [V: Wikipedia Star] |
| Priority services | Priority reservations waitlist and standby (Star Silver); priority check-in, boarding, baggage handling and an extra baggage allowance (Star Gold) | [V: Wikipedia Star] |
| Through check-in and bag transfer | IATCI links, alliance connection teams at hubs ("Star Connection Centers", including Los Angeles) | [V for the centres: 29th-anniversary coverage, May 2026]; IATCI requirement [R] |
| Safety audit | IOSA registration is a stated requirement for oneworld connect partners, and so for members | [V: APEX summary via search] |
| Alliance fare products | The carrier's network must be sellable in the alliance round-the-world fare (stated for oneworld connect and Global Explorer) | [V: same] |
| Branding, co-location, IT connectivity | Alliance livery on some aircraft, moving into shared terminals, connecting to the alliance's IT hub | [V for livery: Wikipedia]; rest [R] |

**Management companies.**

| Alliance | Entity and seat | Notes |
|---|---|---|
| Star Alliance | Star Alliance Services GmbH [R for the entity name], Frankfurt Airport Centre. CEO Theo Panagiotoulias; chairman Michael Rousseau (Air Canada) | A 2023 press report cited on Wikipedia says Star gave up its Frankfurt headquarters; a second management office is in Singapore [R]. Staff of roughly 60-80 [R]. Budget not public. [V for seat and officers: Wikipedia] |
| oneworld | oneworld Management Company, set up May 2000 in Vancouver, moved to New York in 2011 and to Fort Worth in December 2022. Chairmanship rotates among member CEOs | Small central team: vice-presidents for commercial, membership and customer experience, communications, plus a CFO and an IT director [V: Wikipedia] |
| SkyTeam | SkyTeam Central, Amstelveen, Netherlands. Chairman Andres Conesa (Aeromexico); CEO Patrick Roux | [V: Wikipedia] |

**Lighter membership tiers.**

| Programme | Launched | Rule | Members |
|---|---|---|---|
| Star Alliance Connecting Partner | Announced 2016; Juneyao Air joined 23 May 2017 | Bilaterals with at least three members. Star benefits apply only when the partner's flight connects to a Star member, not on point-to-point trips | Juneyao (HO). Thai Smile, Feb 2020 until folded into Thai in Jan 2024. Mango was announced first but never implemented. [V: Wikipedia; CAPA Oct 2016] |
| Star intermodal partner | 1 Aug 2022 | Member airlines put their own flight numbers on trains | Deutsche Bahn; Austrian Federal Railways from March 2025 [V: Wikipedia] |
| oneworld connect | 1 June 2018 | At least three member sponsors, IOSA, participation in Global Explorer. All oneworld elites get priority check-in and boarding; lounge access and extra baggage only for the sponsors' elites | Fiji Airways from 5 Dec 2018, sponsored by American, British Airways, Cathay Pacific and Qantas; it became a full member on 1 April 2025 [V: Wikipedia; View from the Wing; APEX] |
| SkyTeam associate | 2007 | Sponsored by a full member | Air Europa, Copa and Kenya Airways joined 4 Sept 2007. Air Europa, Kenya and TAROM were upgraded to full members in 2010 [V: Wikipedia] |

**Fees.** Not public. Press describes full membership as carrying costs small airlines cannot bear, and the lighter tiers as a way to join "without the full costs of membership" [V: View from the Wing; NerdWallet summary]. ESTIMATE for modelling: one-off joining cost of USD 10-30 million (mostly the carrier's own IT integration, bilateral negotiation, training and rebranding over a 12-18 month joining programme) and annual dues of USD 1-8 million scaled to carrier size. I could not source either figure.

**Alliance fare products** [R unless marked].
- Round-the-world fares priced by mileage tier (Star RTW; oneworld Global Explorer; SkyTeam Go Round the World) or by number of continents (oneworld Explorer). Generic rules: start and end in the same country, exactly one Atlantic and one Pacific crossing, 5-16 stops, backtracking between continents restricted, economy about USD 2,500-6,000 and business USD 5,000-14,000 [V: Wikipedia "Round-the-world ticket"].
- Circle Pacific fares and regional air passes (flat-priced sector coupons by distance band, sold with an international ticket).
- Alliance corporate contracts (one discount agreement across all members) and convention products (one discount code for delegates on any member).
- These fares are prorated under alliance-wide special prorate rules, not the MPA.

**FFP money flows** [R for mechanics; rates not found].
- Accrual: when a member of programme P flies on partner O, O pays P for the miles credited, per mile, invoiced monthly. P books it as a sale of miles.
- Redemption: when P's member redeems on O, P pays O an agreed amount per segment by cabin and region. It is not a prorated fare. O opens partner award seats in dedicated booking classes.
- Rates: an FFP "will normally pay partners an agreed per mile rate" [V: trade report via search]. The only number found is an unverified 2012 forum claim of USD 0.0054 per mile for Delta's payments to partners. Points are typically valued at 1-2 US cents to members, and United and Delta each earned over USD 1 billion from their programmes in 2015 [V: Wikipedia "Frequent-flyer program"]. ESTIMATE for modelling: accrual price 0.8-1.5 US cents per mile between airlines (banks pay more); redemption reimbursement USD 30-80 per short-haul economy segment and USD 250-700 per long-haul business segment.
- Lounge access: per-visit charge invoiced through the clearing house. ESTIMATE USD 25-50 per visit.

### 1(d) Antitrust-immune, metal-neutral joint ventures

**Legal mechanism.** DOT may approve an agreement that substantially reduces competition only if it meets a serious transportation need or achieves important public benefits, and may then exempt the parties from the antitrust laws (49 U.S.C. 41308-41309). DOT has "increasingly insisted on 'full metal neutral' integration" for immunised agreements. Almost every final order carries a five-year review requirement. As of early 2018 there were 24 active immunised alliances; United had received immunity 11 times, Delta 9 times and American 14 times since 1992. Between 1993 and 2009 DOT imposed route carve-outs in eight cases; since about 2008 slot divestitures have replaced them. [V: AAI 2018]

**What a JV changes.**
- Revenue: route-group revenue is "pooled and shared", so each partner is "indifferent as to which carrier transports a given passenger" [V: Brueckner & Singer 2019].
- Revenue sharing versus profit sharing: the EC describes oneworld's transatlantic JV and Star's A++ as revenue-sharing, and the Air France-KLM/Alitalia/Delta JV as profit-sharing with joint management of schedules, pricing and capacity [V: EC IP/10/936, IP/13/456, IP/15/4966]. In a profit-sharing JV the pool is revenue less agreed costs, so a partner is also indifferent to who adds a frequency [R].
- Capacity: partners agree where each flies. Because revenue is shared, each partner has a territorial interest: it wants capacity balanced and dislikes a partner overflying its hub. United's post-2018 European growth in the south-west, away from Lufthansa Group hubs' catchment, is read this way [V: Cranky Flier, Oct 2024].
- Scheduling: partners can de-peak. DOJ's example: after American and Swiss began sharing revenue in 2004, American moved its JFK-Zurich flight from the early evening to 21:00 to serve connections, at a cost to that flight's own profitability [V: DOJ comments on oneworld].
- Inventory: the IATA/American illustration below shows why carve-outs distort revenue management.

| O&D on a transatlantic sector | Fare | Operating carrier retains, 50:50 metal-neutral JV | Rank | Retains if gateway-gateway is carved out | Rank |
|---|---|---|---|---|---|
| Gateway to beyond | 1,000 | 500 | 1 | 500 | 2 |
| Behind to gateway | 800 | 400 | 2 | 400 | 3 |
| Gateway to gateway | 600 | 300 | 3 | 600 | 1 |

[V: IATA Economics 2011, citing American Airlines]

**JV register.**

| JV | Parties | Key dates | Structure and remedies | Provenance |
|---|---|---|---|---|
| Northwest-KLM | NW, KL | KLM put about USD 400m into the 1989 Northwest buyout. US-Netherlands open skies Sept 1992. Immunity granted Nov 1992 (final order Jan 1993 [R]). Enhanced alliance agreement 1997: 10-year term with three-year extensions; Northwest bought back KLM's roughly 19% voting stake for about USD 1.2bn in four tranches to 2000 | Codeshare with revenue divided by a mileage prorate formula; joint fares, joint service mark, shared sales incentives. No carve-outs. The only 1990s immunised alliance with real revenue sharing. Later a 50/50 transatlantic profit share [R] | [V: GAO 1995; Aviation Strategy Nov 1997; DOJ 2009; IATA 2011] |
| United-Lufthansa, then A+ | UA, LH | Immunity 1996 (Order 96-5-27). Revenue sharing from 2003, renamed Atlantic Plus | Carve-outs on Frankfurt-Chicago and Frankfurt-Washington, the only nonstop overlaps. Capacity still rose 75% and 105% on those two routes with local passengers flat | [V: DOJ comments on Star and on oneworld] |
| A++ | UA, CO, AC, LH, later all Lufthansa Group carriers | DOT final order 10 July 2009, over DOJ's objection. Parties had 18 months to show a revenue-sharing formula was in place, after which the Frankfurt carve-outs could go. Toronto-Chicago and Toronto-San Francisco carve-outs stayed | Joint pricing, sales, marketing and revenue sharing ("sales without preference"). EC commitments 23 May 2013 for ten years: slots at Frankfurt and/or New York, fare combinability and SPAs, aimed at premium passengers on Frankfurt-New York. 16 hubs. ITA Airways' addition was still before DOT in Nov 2026 | [V: DOJ 2009; EC IP/13/456; Lufthansa deck; DWU Consulting] |
| SkyTeam transatlantic | DL, AF, KL, AZ; from 2020 DL, AF, KL, VS | First six-way application (2004) rejected. Re-filed and granted in 2008; IATA links the change to the Delta-Northwest and Air France-KLM mergers. JV agreements signed 2009 and 2010. Expanded JV with Virgin Atlantic and without Alitalia launched 3 Feb 2020 | Profit sharing. No carve-outs. EC commitments May 2015 for ten years: slots on Amsterdam-New York and Rome-New York, fare combinability, SPAs, FFP access; Paris-New York for premium passengers. 2020: about 23% of transatlantic capacity, USD 13bn combined revenue, up to 341 peak daily transatlantic flights, 110 nonstop routes | [V: IATA 2011; EC IP/15/4966; Delta Feb 2020] |
| Delta-Virgin Atlantic | DL, VS | Delta bought Singapore Airlines' 49% (2012-13) [R]; immunity application filed 8 April 2013 (docket OST-2013-0068) [V: AAI]; granted later that year [R] | Folded into the 2020 four-way JV | Stake [V: Wikipedia] |
| oneworld transatlantic | AA, BA, IB; later AY, EI | Applications in 1997 and 2001 failed. In 2001-02 DOT proposed approval only with open skies, divestiture of Heathrow slots for 16 daily round trips, and carve-outs on Chicago-London and Dallas-London; the carriers withdrew. Third application 2008. DOJ (21 Dec 2009) predicted fare rises of up to 15% on six routes carrying 2.5 million passengers a year. EC decision 14 July 2010; DOT final order 20 July 2010; operations from autumn 2010 [R] | Revenue sharing. No carve-outs. EC: Heathrow or Gatwick slots for 49 extra weekly return flights to New York, Boston, Dallas and Miami, plus JFK slots, FFP access, fare combinability and SPAs, for ten years. DOT: Heathrow slots for Boston and for any US route (four pairs [R]). Applicants claimed over USD 90m a year in consumer benefits | [V: DOJ comments on oneworld; EC IP/10/936; AAI 2018] |
| United-ANA; American-JAL | UA/CO, NH; AA, JL | DOT tentative approval 6 Oct 2010, final 10 Nov 2010, conditional on US-Japan open skies. Implementation planned for spring 2011 (April 2011 [R]) | Transpacific JVs | [V: CAPA; Aviation Week via search] |
| Delta-Aeromexico | DL, AM | DOT show-cause order 4 Nov 2016, final approval Dec 2016 [R], immunity limited to five years | 24 slot pairs at Mexico City and 4 at JFK divested to low-cost carriers; exclusivity clauses struck. Combined 81% share on Mexico City-JFK. Terminated by DOT 15 Sept 2025, stayed 12 Nov 2025, vacated 20 Aug 2026 | [V: AAI 2018; AirlineGeeks; Eckert Seamans; DWU Consulting] |
| Delta-Korean Air | DL, KE | Launched 1 May 2018 | Transpacific JV. Delta holds 14.9% of Hanjin KAL. Asiana's absorption on 17 Dec 2026 enlarges it | [V: Wikipedia; trade press] |
| Qantas-Emirates | QF, EK | Began 31 March 2013 under ACCC authorisation, renewed since [R] | Revenue-sharing partnership between an alliance member and an unaligned carrier. Listed by both as a JV partner | [V that it is a JV: Wikipedia] |
| Qantas-American | QF, AA | DOT tentatively denied 18 Nov 2016 (nearly 60% of nonstop US-Australasia capacity). Approved on re-application in 2019 [R] | | [V for the denial: AAI 2018] |
| Delta-LATAM | DL, LA | Stake announced 26 Sept 2019. LATAM left oneworld 1 May 2020. DOT approval 2022 [R] | | [V: Wikipedia] |
| Denials worth modelling | | Air France-KLM/Delta/Alitalia with Air Tahiti Nui on Paris-Los Angeles, denied 2013-14; Aloha-Hawaiian inter-island capacity JV, opposed by DOJ 2002 | | [V: AAI 2018] |

Other JVs to seed the simulator [R]: Lufthansa-ANA (Europe-Japan, 2011-12); Lufthansa-Singapore Airlines (2015); Lufthansa-Air China (2016-17); British Airways-JAL with Finnair and Iberia (Europe-Japan); British Airways-Qatar Airways; Air France-KLM with China Southern and China Eastern; Air Canada-Air China; Air New Zealand with United, with Singapore Airlines and with Cathay Pacific; ANA-Singapore Airlines; Virgin Australia-Qatar Airways (2025). American-JetBlue's Northeast Alliance (2021) was struck down by a US court in 2023.

### 1(e) Equity

| Investor | Holdings | Provenance |
|---|---|---|
| Delta | Virgin Atlantic 49%; Wheels Up 21%; Aeromexico about 20% (19% at 30 Sept 2026); Hanjin KAL 14.9-15%; Republic Airways Holdings 14.4%; WestJet 12.7% (2025, alongside Korean Air's 10% and a small Air France-KLM stake); LATAM 10% (bought 20% for USD 1.9bn, diluted in the 2020-22 Chapter 11); Air France-KLM about 3%; China Eastern about 3% | [V: Wikipedia infoboxes for Delta, LATAM, Korean Air; DWU Consulting] |
| Etihad ("equity alliance") | airberlin 29.21% (Dec 2011); Air Seychelles 40%; Aer Lingus 2.987%; Virgin Australia 10%; Jet Airways 24% for USD 379m (2013); Air Serbia 49% (Aug 2013); Darwin Airline 33.3%, rebranded Etihad Regional (2014); Alitalia 49% for about EUR 560m (Aug 2014). "Etihad Airways Partners" launched Oct 2015 and ceased in 2018. Alitalia entered insolvency proceedings 2 May 2017; airberlin filed 15 Aug 2017 after Etihad withdrew funding. Etihad lost USD 1.87bn in 2016 and 1.52bn in 2017, more than USD 5.6bn cumulatively from 2016, and had left every holding by the end of 2023 | [V: Wikipedia "Etihad Airways"] |
| Qatar Airways | IAG 25.1% (from 9.99% in 2015); Cathay Pacific about 10% (Nov 2017, reported divested 2026); LATAM 10%; Virgin Australia 25% (Feb 2025); Airlink 25% (Aug 2024); RwandAir 49% (as stated by Wikipedia; completion doubtful [R]); 49% of Meridiana/Air Italy's holding company (2017, airline liquidated 2020 [R]) | [V: Wikipedia "Qatar Airways"] |
| Lufthansa Group | ITA Airways: agreed 25 May 2023, EC conditional approval 3 July 2024, 41% closed 17 Jan 2025 for about EUR 325m; option on a further 49% exercised May 2026 with completion expected early 2027; option on the last 10%. ITA left SkyTeam 30 April 2025 and joined Star on 1 April 2026, adopting Miles & More | [V: Wikipedia "ITA Airways"] |
| Air France-KLM | SAS: 19.9% in 2024 as part of the restructuring consortium; announced 4 July 2025 that it would go to 60.5% by buying out Castlelake and Lind Invest, the Danish state keeping 26.4%; closing targeted for the second half of 2026. SAS left Star on 31 Aug 2024 and joined SkyTeam on 1 Sept 2024 | [V: Reuters-syndicated coverage via search; Wikipedia "SkyTeam"] |
| Korean Air | Asiana 63.88% (completed 12 Dec 2024, KRW 1.5 trillion); WestJet 10% | [V: Wikipedia "Korean Air"] |

What the record shows: equity is neither necessary nor sufficient for cooperation. KLM's Northwest stake ended in litigation and a buyback while the alliance itself was extended for ten years [V: Aviation Strategy 1997]. Etihad's stakes bought feed but not control, and two investees failed within three months of each other. Delta's stakes are consistently paired with a JV and with the investee leaving or staying out of a rival alliance (LATAM, Virgin Atlantic, Korean, Aeromexico).

---

## 2. Alliance facts

### Founding, size and share

| | Star Alliance | oneworld | SkyTeam |
|---|---|---|---|
| Founded | 14 May 1997 | Announced 21 Sept 1998, operational 1 Feb 1999 | 22 June 2000 |
| Founders | Air Canada, Lufthansa, SAS, Thai, United | American, British Airways, Canadian Airlines, Cathay Pacific, Qantas | Aeromexico, Air France, Delta, Korean Air |
| Full members, Nov 2026 | 26 (25 after Asiana leaves on 16 Dec 2026) | 15, with Hawaiian inside Alaska Air Group since April 2026; Philippine Airlines due as the 16th in 2027 | 18 |
| Network | More than 1,150 airports in 190 countries (May 2026). About 1,300 airports and over 19,000 daily departures on 2019 figures | About 1,000 airports in 170 territories and over 13,000 daily departures (March 2020) | Over 1,000 destinations in 160+ countries. Daily flights quoted between 13,800 and 17,300 by secondary sources |
| Destinations, year to May 2023 (OAG) | 1,289 | 998 | 1,058 |
| Passengers, 2019 | 762 million | 535 million | 630 million (624 million in 2024) |
| Share of world scheduled RPKs, 2012 | 25.2% | 14.1% | 19.8% |
| 2014 | 23.7% | 18.3% | 20.5% |
| 2018 | 21.9% | 15.4% | 18.8% |
| 2020 | 18.7% | 12.7% | 16.3% |
| "Market share", April 2024 (Statista as cited by Wikipedia; measure not stated) | 17.4% | 11.9% | 13.7% |
| Largest member's share of the alliance's seats, Dec 2022 (OAG) | United, about 21% | American, 41% | Delta, 30% |
| Management | Frankfurt (and Singapore [R]) | Fort Worth since Dec 2022 | Amstelveen |

[V: Wikipedia alliance pages; IATA WATS press releases for 2012, 2014, 2018 and 2020 via search; OAG May 2023 and Dec 2022; Star 29th-anniversary coverage, May 2026]

Aligned versus unaligned:

| Date | Measure | Three alliances | Low-cost | Other unaligned | Source |
|---|---|---|---|---|---|
| Oct 2011 | ASKs | 56.9% (25.3 + 17.8 + 13.8) | 17% | 26.2% | CAPA/Innovata [V] |
| 2014 | RPKs | 62.5% | | | IATA WATS [V] |
| 2018 | RPKs | 56.1% | | | IATA WATS [V] |
| 2019 | seats | 45.9% | | | OAG [V] |
| 2020 | RPKs | 47.7% | | | IATA WATS [V] |
| Nov-Dec 2022 | seats / ASKs | 43% | | 57% non-aligned in total | OAG; Statista on OAG data [V] |
| Year to May 2023 | seats | "close to 45%"; Star 17% | 31% unaligned low-cost | | OAG [V] |
| April 2024 | not stated | 43.0% | | | Statista via Wikipedia [V] |

Two readings for the simulator. First, the alliances' share is much higher on long-haul than overall: by mid-2011 they provided over 80% of capacity across the Atlantic and the Pacific and just under 80% between Europe and Asia [V: IATA Economics 2011]. Second, RPK shares run about 10 points above seat shares because alliance members fly longer sectors, so "55-60% aligned" and "43% aligned" can both be true of the same year depending on the measure. Model the share as an output of the network, not as a parameter.

### Members by IATA code (as of 18 Nov 2026)

**Star Alliance (26):** A3 Aegean, AC Air Canada, CA Air China, AI Air India, NZ Air New Zealand, NH ANA, OZ Asiana (leaves 23:59 KST 16 Dec 2026), OS Austrian, AV Avianca, SN Brussels Airlines, CM Copa, OU Croatia Airlines, MS EgyptAir, ET Ethiopian, BR EVA Air, AZ ITA Airways (joined 31 Mar / 1 Apr 2026), LO LOT, LH Lufthansa, ZH Shenzhen Airlines, SQ Singapore Airlines, SA South African Airways, LX SWISS, TP TAP Air Portugal, TG Thai, TK Turkish Airlines, UA United. Connecting partner: HO Juneyao. Intermodal: Deutsche Bahn, OBB. [V: Wikipedia]

**oneworld (15 full members):** AS Alaska, AA American, BA British Airways, CX Cathay Pacific, FJ Fiji Airways, AY Finnair, IB Iberia, JL Japan Airlines, MH Malaysia Airlines, WY Oman Air, QF Qantas, QR Qatar Airways, AT Royal Air Maroc, RJ Royal Jordanian, UL SriLankan. Hawaiian (HA) joined on 22-23 April 2026 as part of Alaska Air Group; Wikipedia's member table lists it as a 16th member while the oneworld count treats Philippine Airlines (PR, 2027) as the 16th. Suspended: S7. [V: Wikipedia]

**SkyTeam (18):** AR Aerolineas Argentinas, AM Aeromexico, UX Air Europa, AF Air France, CI China Airlines, MU China Eastern, DL Delta, GA Garuda Indonesia, KQ Kenya Airways, KL KLM, KE Korean Air, ME Middle East Airlines, SV Saudia, SK SAS, RO TAROM, VN Vietnam Airlines, VS Virgin Atlantic, MF XiamenAir. Suspended: SU Aeroflot. Eurostar signed an MoU in Sept 2024 to become the first non-airline partner. [V: Wikipedia]

### Entries, exits and defections

| Carrier | Move | Date | Reason | Provenance |
|---|---|---|---|---|
| Canadian Airlines | Left oneworld | 1 June 2000 | Acquired by Air Canada (Star) | [V] |
| Ansett Australia | Star 1999-2001 | Collapsed Sept 2001 | Bankruptcy; left Star without an Australian member | [V: years] |
| Mexicana | Left Star 2004; oneworld 10 Nov 2009 to 28 Aug 2010 | | Codeshare dispute with United; later ceased operations | [V] |
| Varig | Star 1997-2007 | 2007 | Collapse and restructuring | [V: years] |
| Aer Lingus | Left oneworld | 31 Mar 2007 | Shift to a low-fare point-to-point model. Now in the transatlantic JV without being a member | [V] |
| Continental | SkyTeam to Star | Left 24 Oct 2009, joined Star 27 Oct 2009 | After Delta-Northwest, Continental chose United (alliance announced June 2008, merger 2010) | [V] |
| Copa | SkyTeam associate to Star | Left 24 Oct 2009; Star 2012 | Followed Continental | [V] |
| Northwest | Left SkyTeam | 31 Jan 2010 | Merged into Delta | [V] |
| Shanghai Airlines | Left Star | 2010 | Merged with China Eastern (SkyTeam) | [V] |
| Malev | oneworld 29 Mar 2007 to 3 Feb 2012 | | Ceased operations | [V] |
| Spanair, bmi | Left Star | 2012 | Ceased operations; bmi absorbed by British Airways | [V] |
| TAM | Star to oneworld | Left Star 2014 | LAN-TAM merger chose oneworld | [V] |
| US Airways | Star to oneworld | Joined oneworld 31 Mar 2014 | Merger with American | [V] |
| airberlin | oneworld 20 Mar 2012 to 28 Oct 2017 | | Insolvency after Etihad stopped funding | [V] |
| China Southern | Left SkyTeam | 31 Dec 2018 | Not stated. It then deepened ties with American and other oneworld carriers [R] but has not joined | [V: date] |
| Adria | Left Star | 2019 | Ceased operations | [V] |
| LATAM | Left oneworld | Announced 31 Jan 2020, effective 1 May 2020 | Delta's 20% stake and planned JV | [V] |
| Royal Air Maroc | Joined oneworld | 1 Apr 2020 | | [V] |
| Alaska | Joined oneworld | 31 Mar 2021 | American partnership | [V] |
| Alitalia, then ITA | Alitalia in SkyTeam 27 July 2001 to 15 Oct 2021; ITA in SkyTeam 29 Oct 2021 to 30 Apr 2025; ITA in Star from 1 Apr 2026 | | Lufthansa's 41% stake | [V] |
| Aeroflot | SkyTeam membership suspended | 28 Apr 2022 | Invasion of Ukraine | [V] |
| S7 | oneworld membership suspended | 19 Apr 2022 | Same | [V] |
| Virgin Atlantic | Joined SkyTeam | 2 Mar 2023 | Delta's 49% and the JV | [V] |
| SAS | Star to SkyTeam | Left 31 Aug 2024, joined 1 Sept 2024 | Air France-KLM-led restructuring investment | [V] |
| Czech Airlines | Left SkyTeam | 26 Oct 2024 | Wound down | [V] |
| Fiji Airways | oneworld connect to full member | 1 Apr 2025 | | [V] |
| Oman Air | Joined oneworld | 30 June 2025 | Sponsored by Qatar Airways; MoU June 2022 | [V] |
| Hawaiian | Joined oneworld | 22-23 Apr 2026 | Acquired by Alaska Air Group (closed 18 Sept 2024) | [V] |
| Asiana | Leaves Star | 16 Dec 2026; merges into Korean Air 17 Dec 2026 | Korean Air took 63.88% on 12 Dec 2024. Moves about 80 aircraft, 14 million frequent flyers and 90 international routes into SkyTeam | [V: ch-aviation and others via search; Global Traveler] |
| Philippine Airlines | To join oneworld | MoU June 2026, entry 2027 | | [V] |
| Starlux | oneworld application stalled | May 2026 | Reported objection from Cathay Pacific | [V: Wikipedia; Global Traveler] |
| TAP | Possible Star to SkyTeam | Pending | Sale of a 44.9% stake; Lufthansa Group and Air France-KLM the remaining bidders | [V: Global Traveler] |

Patterns for the simulator's event model: (1) a merger or controlling stake across alliances forces a switch, usually 12-18 months after closing; (2) failures remove members abruptly; (3) a founding member can veto a competitor in its home market; (4) sanctions suspend rather than expel; (5) leaving "can take at least a year to untangle" [V: OAG Dec 2022].

### Unaligned carriers

| Carrier | Why unaligned | Provenance |
|---|---|---|
| Emirates (EK) | Considered Star in 2000 and stayed out. A senior executive said that needing a consensus among alliance partners slows a carrier's reaction in the market. Builds bilaterals instead: 39 codeshare agreements (35 airlines, 4 rail operators), with Qantas as a JV partner | [V: Wikipedia] |
| Low-cost carriers: Southwest (WN), Ryanair (FR), easyJet (U2), Wizz (W6), IndiGo (6E), AirAsia (AK), and others | Point-to-point models with no interline systems or obligations; alliance service standards add cost. Unaligned low-cost carriers were 31% of world seats in the year to May 2023. IndiGo and JetBlue do codeshare selectively | [V for the 31%: OAG]; rationale [R] |
| China Southern (CZ), LATAM (LA), Etihad (EY), JetBlue (B6), Virgin Australia (VA), WestJet (WS), Hainan (HU), Aer Lingus (EI), Icelandair (FI) | Prefer bilateral codeshares, JVs and equity links to alliance membership. JetBlue is the largest unaligned US carrier, with a partnership with United since May 2025 | [V for JetBlue: Global Traveler]; rest [R] |
| Former alliances | Value Alliance and U-FLY (both 2016, low-cost, defunct); Vanilla Alliance (2015, Indian Ocean); earlier Qualiflyer and Wings | [V: Wikipedia; IATA 2011] |

### Principal hubs [R]

| Alliance | Hubs |
|---|---|
| Star | UA: ORD, DEN, IAH, EWR, SFO, IAD, LAX. AC: YYZ, YUL, YVR. LH: FRA, MUC. LX: ZRH. OS: VIE. SN: BRU. AZ: FCO. TP: LIS. LO: WAW. TK: IST. A3: ATH. MS: CAI. ET: ADD. SA: JNB. SQ: SIN. TG: BKK. NH: HND, NRT. BR: TPE. CA: PEK, CTU. ZH: SZX. AI: DEL, BOM. NZ: AKL. AV: BOG. CM: PTY |
| oneworld | AA: DFW, CLT, MIA, ORD, PHL, PHX, JFK, LAX. AS: SEA, PDX, HNL. BA: LHR. IB: MAD. AY: HEL. QR: DOH. RJ: AMM. AT: CMN. WY: MCT. CX: HKG. JL: HND, NRT. MH: KUL. UL: CMB. QF: SYD, MEL. FJ: NAN |
| SkyTeam | DL: ATL, DTW, MSP, SLC, JFK, BOS, SEA, LAX. AM: MEX. AR: EZE. AF: CDG. KL: AMS. VS: LHR. SK: CPH. UX: MAD. RO: OTP. ME: BEY. SV: JED, RUH. KQ: NBO. KE: ICN. MU: PVG. MF: XMN. CI: TPE. VN: HAN, SGN. GA: CGK |

The Delta/Air France-KLM/Virgin JV lists its hubs as Amsterdam, Atlanta, Boston, Detroit, London Heathrow, Los Angeles, Minneapolis, New York JFK, Paris, Seattle and Salt Lake City [V: Delta, Feb 2020]. The Star Atlantic JV counts 16 hubs [V: Lufthansa deck].

---

## 3. Measured economic effects

### Interline (behind-and-beyond) fares: the double-marginalisation result

Theory: two non-aligned carriers each add a markup to their part of an interline fare and ignore the lost traffic on the other's sector. Cooperation internalises this. A JV should bring the fare down to the single-carrier level; partial cooperation gets part of the way. [V: Brueckner & Singer 2019]

| Study | Data | Result (relative to non-aligned interline unless stated) |
|---|---|---|
| Brueckner & Whalen (2000), J. Law & Econ. 43(2) | DB1B, 1997 Q3 cross-section | Allied (codesharing) carriers charge about 25% less. Gateway-to-gateway: about +5% when two previous competitors ally, not significant [V] |
| Brueckner (2003), REStat 85(1) | DB1B, 2000 | Codeshare -8% to -17%; immunity -13% to -21%; combined -17% to -30% because the two are substitutes [V: abstract via search] |
| Brueckner (2003), companion paper | Same | Codeshare -7%, alliance membership a further -4%, immunity a further -16%: -27% in total [V: as restated in Brueckner & Singer 2019] |
| Whalen (2007), Rev. Ind. Org. 30 | US-Europe panel, 1990-2000 | Codeshare -9% and a further -18% from immunity: -27% in the preferred specification. Immunised fares often statistically equal to online fares. Large increases in passenger volume. Open-skies capacity growth came entirely from immunised carriers between their hubs [V: abstract; Brueckner & Singer 2019] |
| Brueckner, Lee & Singer (2011), J. Comp. Law & Econ. 7(3) | DB1B panel, 1998-2009 | US-world, all passengers: codeshare -3.6%, alliance -2.7%, immunity -4.9%, full cooperation -11.2%, online -14.4%. Transatlantic: -3.9%, -7.6%, -4.4%, total -15.9%, online -18.9%. Economy only: -9.5% and -12.6%. Business: -8.1% and -11.4% [V: IATA Economics, Oct 2010] |
| DOJ (2009), Star comments, Appendix | DB1B, 2008 Q3, US-Europe | Relative to online: interline +6.3%, non-immunised alliance -1.5% (not significant), immunised alliance +2.1%. So immunity adds nothing over alliance membership [V] |
| DOJ (2009), oneworld comments | | Immunised fares 6.0-16.5% above unimmunised fares within each alliance [V: as summarised by IATA Economics 2010] |
| Gillespie & Richard (2011), DOJ EAG 11-1 | DB1B, 2005-2010, 1.93 million economy tickets | Online 3.3% (oneworld), 5.2% (SkyTeam), 7.6% (Star) cheaper; non-immunised alliance 1.3-2.1% cheaper; immunised between -1.8% and +1.7%. Immunity "not reasonably necessary" for the saving [V] |
| Calzaretta, Eilat & Israel (2017) | DB1B, 1998-2016 | Online and JV itineraries both about 8% cheaper; no gateway-to-gateway harm [V: via Brueckner & Singer 2019] |
| Brueckner & Singer (2019), Econ. of Transportation 20 | DB1B plus confidential foreign-carrier data, 1997-2016 | Online -7.4%; alliance without immunity -0.7%; immunity without JV -6.4%; JV -7.2%, statistically equal to online. Open skies -6.7%. A low-cost carrier on the connection -5.3%. One more connecting competitor -0.3% [V] |

The disagreement between the Brueckner line and the DOJ line turns on whether to control for carrier identity [V: IATA Economics 2011]. What both lines agree on: non-aligned interline is the most expensive way to connect, and online or JV pricing is the cheapest. The size of the gap has shrunk from about 25% in 1997-2000 data to 7-15% in panels reaching the 2010s. Brueckner, Lee and Singer suggest non-aligned carriers cut their own interline fares in response [V: Brueckner & Singer 2019].

### Gateway-to-gateway (nonstop overlap) fares

| Source | Result |
|---|---|
| DOJ (2009), 2008 Q3 | Nonstop fares rise about 15% when nonstop competitors fall from two to one, 6.6% from three to two, 6.3% from four to three. 73% of coach passengers fly nonstop where it exists even though connecting fares average 10% less [V] |
| DOJ on oneworld | Up to 15% on Boston-, Chicago-, Dallas-, Miami- and New York-London and Miami-Madrid. Nonstop fares on those six routes average 28% above connecting fares [V] |
| Gillespie & Richard (2011) | Each lost independent nonstop competitor raises fares about 7% (about USD 31 one way). An added immunised partner has no effect, so partners do not count as competitors. 76% of economy passengers fly nonstop; nonstop fares are 8.3% above connecting [V] |
| Brueckner & Singer (2019) | An extra competitor lowers fares 4.0% (1997-2009) and 7.0% (2010-2016). In 2010-2016 each added immunised pair raises fares 6.8% and each added JV pair 4.6% (7% on average; 7.8% for JVs when passenger-weighted). Granting immunity or JV status to two previously unaligned carriers equals removing a competitor. A low-cost carrier on the route lowers fares 10%. Carve-outs have no restraining effect and covered about 1% of itineraries [V] |
| Brueckner & Whalen (2000); Calzaretta et al. (2017) | No significant gateway-to-gateway effect [V] |
| Bilotkach & Huschelrath (2015) | JVs raised hub-to-hub capacity 3-5% at the expense of the rest of the network; load factors 0.5-5% lower on JV routes than on immunity-only routes [V: via AAI 2018] |

Net effect: Brueckner & Singer simulate removing a JV on two routes in 2015 Q3. Gateway passengers would save USD 2.06 million on route A and USD 0.47 million on route B; connecting passengers would pay USD 3.10 million and USD 10.81 million more. The JV is beneficial on balance, less so where the gateway market is thick. Their policy conclusion is that slot divestiture which brings entry works and carve-outs do not. [V]

### Traffic and revenue gains

| Alliance | Measured gain | Provenance |
|---|---|---|
| Northwest-KLM | 353,000 passengers on Northwest aircraft under the alliance in the year to June 1994, against 164,450 interline connecting passengers in 1991, plus about 150,000 on KLM-only codeshares. Northwest's added 1994 revenue USD 125-175 million, about a third of its USD 455 million transatlantic passenger revenue; KLM about USD 100 million. Combined transatlantic share from 7% to 11.5%; share in 34 US and 30 European and Middle Eastern interior cities from 1.2% to 3.3%. KLM's own estimate of alliance benefit: NLG 100-150 million a year | [V: GAO 1995; Aviation Strategy 1997] |
| British Airways-USAir | BA about USD 100 million (Apr 1994 to Mar 1995), USAir about USD 20 million. Competitors lost: Delta about USD 25 million in 1994; one US airline over USD 40 million, 11% of its transatlantic operating revenue | [V: GAO 1995] |
| United-Lufthansa | About 600 extra passengers a day in the first year from June 1994, projected to reach 1,000 a day | [V: GAO 1995] |
| United-Ansett; United-British Midland | About 43,800 passengers and USD 14 million a year; monthly codeshare passengers from 151 to 2,072 | [V: GAO 1995] |
| Carved-out hub routes | Chicago-Frankfurt capacity +75% and Washington-Frankfurt +105% after 1996 with local traffic flat: the growth was connecting flow | [V: DOJ] |
| JV scale | Delta/Air France-KLM/Virgin: USD 13 billion a year and 23% of transatlantic capacity (2020). Star Atlantic JV: about 30% transatlantic share by its own account | [V: Delta; Lufthansa document via search] |
| US majors within their alliances | Share of alliance traffic on US-international segments, 2008 to 2016: United 35% to almost 60% of Star, American 47% to 67% of oneworld, Delta 37% to 56% of SkyTeam | [V: AAI 2018, T-100 data] |

Not found: a current airline statement of the form "the alliance contributes X% of revenue". Older academic estimates I recall but did not verify [R]: Oum, Park & Zhang (1996) found that codesharing between non-leader carriers on transpacific routes cut the market leader's fare by about USD 83 and raised its traffic by about 10,000 passengers a year; Park & Zhang (2000) found alliance partners' traffic rose by about 36,000 passengers a year per route and fares fell about USD 41 on North Atlantic alliance routes; Bamberger, Carlton & Neumann (2004) found fares fell 5-7% and traffic rose after two US domestic alliances.

### Mix of connecting passengers: online, alliance, interline

| Source and sample | Online | Immunised / JV | Alliance, not immunised | Non-aligned interline |
|---|---|---|---|---|
| 1990s, US-EU connecting passengers [V: Gillespie & Richard] | | | | 38% |
| Brueckner & Singer, share of connecting itineraries, 1997 [V] | | under 10% (JV) | | about 40% |
| DOJ, US-Europe tickets, 2008 Q3 [V] | 67.8% | 17.4% | 7.4% | 7.4% |
| Gillespie & Richard, 2005-2010 [V] | 56% | 21% | 14% | 9% |
| Brueckner & Singer, share of connecting itineraries, 2016 [V] | falling slightly after 2010 | about 30% (JV) | | about 10% |
| IATA, all itineraries, 2018 [V] | | | | about 10% of itineraries involve another airline; over 8% of segments were sold by another airline |

Codeshare prevalence: 25% of published flights are marketing codeshare flights (2017) [V: IATA]. I found no carrier-level figure for the share of bookings made under a codeshare flight number. ESTIMATE for a network carrier: 5-12% of passengers.

Supporting demand evidence: a 2006 Oxford Economics survey of 600 businesses for IATA found over 90% considered transfers within one airline or one alliance acceptable, and connections between non-allied airlines much less so [V: IATA Economics 2011].

---

## 4. Operational and system behaviour worth simulating

### Irregular operations and involuntary reroute (IATA Resolution 735d)

[V: IATA guidance document, June 2020, unless marked]

- Trigger: a flight cancellation, failure to operate reasonably to schedule, a missed stop, inability to provide confirmed space, or a misconnection caused by the carrier, arising no earlier than one day before the scheduled departure of the first affected flight. Earlier changes are planned schedule changes under Recommended Practice 1735 and are handled differently.
- Responsibility: the original operating carrier, the one that caused the disruption, must either refund (Resolution 737) or provide onward carriage at no extra charge: on its own aircraft, on the original receiving carrier, or on any other transport service. On a misconnection the delivering airline cancels the unusable onward space and rebooks (Resolution 766). It also bears hotel, meals, ground transport and communication costs (735d Article 6).
- Getting seats on another airline: follow the bilateral method if one exists (the AIRIMP "NI" action code, direct access, sell-and-report, or a telephone call), within the bilaterally agreed number of seats. Without a bilateral, the disrupted carrier must contact the new carrier first. No upgrade of cabin unless bilaterally agreed.
- Ticketing: request control of only the affected coupons with an involuntary indicator; only the validating carrier can release control. Reissue with "INVOL" as the first five characters of the endorsement box, carry forward the original fare calculation, fare, taxes, fare basis of the first affected coupon, and baggage allowance. Applies to all fare types including award and bulk tickets.
- Ancillaries do not transfer to another airline unless bilaterally agreed (RP 1790); the original carrier refunds.
- Billing: the new carrier bills under Revenue Accounting Manual chapter A2. A reissue within two days of the first affected departure is presumed a valid involuntary reroute; a coupon departing five or more days after the reissue is not. Proration runs twice, as described in section 1(a). Taxes follow the new routing. A multilateral "Simplified Involuntary Reroute Settlement" run by the Airlines Clearing House with ATPCO and Accelya applies precomputed billing values.
- Under MITA, parties are bound by 735d, so any MITA partner is a legal reroute option. In practice [R] carriers apply a priority order: own flights; JV partners, whose seats are economically their own; alliance partners, with whom inventory methods and airport rebooking tools already exist; then other interline partners, often only for premium customers or long delays; then refund. The order is commercial policy, not part of the resolution. A carrier with no interline agreement, such as most low-cost carriers, can only rebook on itself or refund.

### Through check-in, baggage and connections

- Through check-in needs IATCI links between departure control systems, bilateral by carrier pair [R]. Alliance membership makes the full mesh mandatory; plain interline leaves it optional.
- Alliances staff connection centres at hubs to protect short connections and reroute bags: Star Connection Centers, including Los Angeles [V: May 2026 coverage].
- Minimum connecting time is filed per airport by carrier pair, terminal pair and international/domestic status. For a codeshare flight the operating carrier's MCT governs unless the marketing carrier files its own [V: IATA MCT Technical Guide]. Alliances lower effective MCTs by co-locating: Star's "Move Under One Roof" programme put members in one terminal at airports such as London Heathrow Terminal 2 and Tokyo Narita Terminal 1 [R]. Model co-location as a carrier-pair MCT exception below the airport's inter-terminal default.
- Reciprocal status benefits to simulate: priority waitlist and standby clearance (Star Silver and above), lounge access, priority check-in, boarding and baggage delivery, and an extra bag (Star Gold) [V: Wikipedia]. Equivalent tiers: oneworld Ruby, Sapphire, Emerald; SkyTeam Elite, Elite Plus [R].

### Availability and inventory between partners

- Free-sale codeshare availability is the operating carrier's, filtered through the RBD mapping table. The marketing carrier may apply its own point-of-sale and O&D controls on top.
- Partners map each other's fares into their own revenue management so a seat request for a partner-ticketed connecting passenger is valued as if it were their own [V: DOJ]. In a JV the pooled-revenue value replaces the prorated value, which removes the bias against shared-revenue passengers [V: IATA Economics 2011 table in section 1(d)].
- GDS neutral displays rank online and codeshare connections above interline ones [V: IATA]. This is the main reason codeshare changes bookings even when the physical product is identical.
- Award space is a separate inventory bucket that the operating carrier opens to partners. Star Alliance carriers use X (economy), I (business) and O (first) for partner awards; oneworld commonly T, U and Z; SkyTeam X and O [R]. Carriers often release fewer seats to partners than to their own members.

### Settlement shocks

- A carrier failure or clearing-house suspension leaves partners with unpaid coupon billings and stranded passengers holding the failed carrier's tickets (Ansett 2001, Malev 2012, airberlin 2017, Alitalia 2021).
- Commercial disputes end partnerships at short notice: Mexicana left Star in 2004 over a codeshare dispute; Qatar Airways threatened in December 2018 to leave oneworld, accusing Qantas and American of "hostile business practices" [V: Wikipedia]. FFP rate disputes are a known cause of partners being dropped from earn-and-burn [R].
- Sanctions: Aeroflot and S7 were suspended in April 2022 and partners had earlier pulled codeshares [V].
- Regulatory reversal: the Delta-Aeromexico sequence of terminate, stay and vacate in 2025-26 [V].

### Alliance IT

- Star Alliance chose Amadeus in 2005 to build a common IT platform (reservations, inventory and departure control on what became Altea); Lufthansa and several European members migrated, United did not [R, not verified]. The lesson for a model is that alliances standardise interfaces, not systems: each member keeps its own PSS and connects through bilateral or hub messaging.
- Current alliance projects align booking systems, airport operations, transfer handling and loyalty integration [V: May 2026 coverage].

---

## 5. Regional feed partnerships: a different contract family

The alliance contracts above divide revenue between two airlines that each sell seats. Regional feed contracts decide whether the operator sells seats at all.

### US capacity purchase agreements (CPAs)

| Term | SkyWest (FY2023 10-K) | Mesa (FY2019 10-K) |
|---|---|---|
| Payment | Fixed fee per departure, per flight hour and per block hour, plus an amount per aircraft in service each month, part of which is treated as aircraft lease revenue | Guaranteed monthly amount per aircraft, plus fixed fees per block hour and per flight |
| Incentives | Flight completion, on-time performance, other metrics | American: controllable on-time departures and completion. United: also passenger satisfaction surveys, with a minimum margin tied to performance |
| Who controls the product | United, Delta, American and Alaska "controlled scheduling, ticketing, pricing, and seat inventories" | Same |
| Fuel, landing fees, airport rents | Paid directly or reimbursed by the major. The majors "bear the economic risk of fuel price fluctuations" on contract flying | Fuel and certain landing fees paid directly by the major. Insurance, property tax and passenger liability reimbursed at cost: USD 37.2 million, 5.8% of non-fuel operating cost |
| Revenue risk | Major. SkyWest gains nothing from higher fares or lower fuel prices | Major |
| Cash | Weekly provisional payments, reconciled monthly or quarterly | |
| Term | Average remaining term 2.7 years (United), 4.8 (Delta), 3.1 (American), 7.5 (Alaska) | American tranches expiring 2021-2025; United may remove aircraft on 90 days' notice subject to conditions |
| Scale | 603 aircraft, 485 in scheduled service, about 1,850 daily departures: 740 United Express, 580 Delta Connection, 340 American Eagle, 190 Alaska | 62 CRJ-900 for American; 20 CRJ-700 and 60 E-175 for United |
| Revenue | Flying agreements USD 2,834 million: CPA flight operations 1,977, fixed aircraft lease 296, variable lease 180, prorate and charter 381. CPA is 86.5% of the total | Contract revenue USD 682.8 million on 456,247 block hours and 246,634 departures; 6.29 cents per ASM |

[V: SEC filings]

Derived rates (ESTIMATE, arithmetic on the verified figures above):
- Mesa FY2019: USD 1,497 per block hour, or USD 2,769 per departure, excluding fuel and pass-through costs. Forty-two of Mesa's 60 E-175s were leased in rather than owned [V], so this is closer to a pure operating fee than SkyWest's figure.
- SkyWest 2023: total flying-agreement revenue divided by roughly 675,000 annual departures (1,850 a day) is about USD 4,200 per departure, including aircraft ownership compensation and excluding partner-paid fuel.

Further contract mechanics from the 2025 United/Republic form CPA, where the rates themselves are redacted [V: Law Insider exhibit]: flights cancelled by the major are paid as if operated; controllable and uncontrollable cancellations are defined separately and treated differently; diversions are paid on actual block hours; invoices are paid in four instalments a month with a monthly true-up; the major reimburses capital modifications, livery and safety audits as pass-through costs; a spare-aircraft ratio is specified. Mesa's FY2018 schedule credits to American for missed performance were USD 5.2 million [V]. Mesa ended its American contract on 3 April 2023 because pilot wage increases were not recoverable under the fixed fee and utilisation penalties applied [V: 10-K summaries via search].

### Prorate (revenue-sharing) regional agreements

SkyWest's United and Delta prorate agreements: the operator receives a negotiated share of each connecting passenger's fare and the whole fare on local traffic, pays all costs including fuel, and bears the revenue risk. They are terminable on 120 days' notice (United) and 30 days' notice (Delta). Prorate and charter flying was 13% of SkyWest's 2023 flying revenue, on about 29 aircraft. A 25% fuel price rise would have cost about USD 21.5 million on that flying. Some prorate routes carry Essential Air Service subsidy on two-year contracts. [V: SkyWest FY2023 10-K]

### Scope clauses

Mainline pilot contracts limit what the major may outsource. Since 2012 the three US majors cap regional jets at 76 seats and 86,000 lb maximum take-off weight. [V: Wikipedia "Scope clause"; Cranky Flier]

| Carrier | Up to 50 seats | 51-70 seats | 71-76 seats | Other limits |
|---|---|---|---|---|
| American | No separate limit | Within the totals at right | Aircraft of 66-76 seats capped at 40% of the mainline narrowbody fleet | All outsourced regional aircraft capped at 75% of the mainline narrowbody fleet. Ex-US Airways CRJ-900s and E175s grandfathered at 79-80 seats |
| Delta | Up to 125 | 102 | 223 (153 plus 70 unlocked by adding small mainline narrowbodies, the 717 and A220) | 85% of regional flying under 900 miles; 90% to or from hubs. A 2025-26 pilot grievance alleged the 223 cap was exceeded |
| United | Up to 90% of the single-aisle fleet | 255 aircraft of 51-76 seats in total | No more than 153 of the 255 | Regional block hours must stay below mainline block hours |

Consequence for fleet modelling: the Embraer E175-E2 and the cancelled Mitsubishi SpaceJet M90 exceed the weight limit, so the first-generation E175 is the only in-production jet that fits US scope. Bombardier launched the CRJ550, a 50-seat CRJ700, in 2019 to fit the 50-seat category. [V]

Ownership overlay: Delta holds 14.4% of Republic Airways Holdings [V: Wikipedia]. Republic merged with Mesa in November 2025 [V: filing index via search]. American and Delta also own regional operators outright (Envoy, PSA, Piedmont; Endeavor) [R].

### European equivalents [R, none verified this session]

| Model | Examples | Who bears revenue risk |
|---|---|---|
| Franchise | British Airways franchisees flying in BA colours under BA codes (historically GB Airways, BMED, Loganair, Comair of South Africa, Sun-Air of Scandinavia); Air Nostrum as Iberia Regional; Emerald Airlines as Aer Lingus Regional | The franchisee. It keeps ticket revenue, pays the brand owner a fee, and sells through the brand owner's inventory system |
| Wet lease (ACMI) | CityJet flying for SAS and others; Lufthansa's former regional partners (Eurowings, Augsburg Airways, Contact Air) flying as Lufthansa Regional | The mainline carrier. It pays per block hour and keeps all revenue. This is the closest European match to a US CPA |
| Owned subsidiary | KLM Cityhopper, Lufthansa CityLine, Air Dolomiti, BA CityFlyer | The group |

The modelling decision is the same in every case: one party holds the inventory and the revenue risk, and the contract fixes whether the operator is paid per unit of flying (CPA, wet lease) or per passenger (prorate, franchise).

---

## Model recommendations

Everything numeric in this section is an ESTIMATE chosen to sit inside the verified ranges above, unless a source is named. Defaults are starting points for calibration.

### A. Data model for partnership agreements

One base record, six subtypes. Agreements are directional where the real ones are (who may ticket whom, who operates, who markets), dated, and scoped to a market so that a carrier pair can hold several at once.

**Agreement (base)**

| Field | Notes |
|---|---|
| id, type | INTERLINE, SPA, CODESHARE, ALLIANCE_MEMBERSHIP, JOINT_VENTURE, CAPACITY_PURCHASE, plus FFP_PARTNERSHIP and EQUITY_STAKE as attachments |
| parties and roles | Ticketing/carrying, marketing/operating, member/alliance, major/operator |
| scope | Market filter: route list, country or region pair, point of sale, cabin, RBD set, date range |
| effective, discontinue, notice_days | |
| settlement_channel | ICH, ACH, direct |
| status | ACTIVE, SUSPENDED, TERMINATED; with reason code |

**InterlineAgreement**

| Field | Default | Basis |
|---|---|---|
| basis | MITA or BILATERAL | [V] |
| direction | Both ways; allow one-way | [R] |
| e_ticketing | true (needs its own bilateral) | [V: BIETA] |
| baggage_through_check | true | [V] |
| iatci_through_checkin | false for plain interline, true inside an alliance | ESTIMATE |
| proration | MPA unless an SPA matches | [V] |
| interline_service_charge_pct | 9 | [V, secondary source] |
| irops_inventory | method NONE, MESSAGE or DIRECT_SELL; max_seats_per_flight 0 without a bilateral, 9 with one | Structure [V: IATA]; the 9 is an ESTIMATE |
| billing_cycle | Monthly billing, weekly clearance | [V: ICH weekly claims] |

**SpecialProrateAgreement**

| Field | Default | Basis |
|---|---|---|
| applies_to | Sector or O&D list, RBDs, point of sale, season | [R] |
| method | FIXED_AMOUNT, PCT_OF_THROUGH_FARE, PCT_OF_LOCAL_FARE, or SRP_WITH_FLOOR_CEILING | [R] |
| value | Short-haul feeder sector under PCT_OF_LOCAL_FARE: 50-70% of the lowest local fare in the mapped class | ESTIMATE |
| interline_service_charge_pct | 5-7; 0 within a JV | [V, secondary source] |
| fallback | MPA straight-rate proration with provisos | [V] |

**CodeshareAgreement**

| Field | Default | Basis |
|---|---|---|
| operating, marketing[] | A flight carries 2.4 marketing numbers on average | [V: IATA] |
| mode | FREE_SALE. Also CAPPED_FREE_SALE, SOFT_BLOCK, HARD_BLOCK | [V] |
| rbd_map | Marketing RBD to operating RBD, many-to-one allowed | [V: concept] |
| availability_method | AVS_STATUS for distant partners; INTERACTIVE_POLL for alliance and JV partners | [R] |
| cap_seats | Capped free sale: 20-30% of cabin | ESTIMATE |
| block_seats, block_price | Hard block: 10-20% of cabin at 75-85% of the operator's average sold fare for that cabin | ESTIMATE |
| release_days | Soft block: unsold seats returned 14-30 days before departure | ESTIMATE |
| commission_pct | Free sale: 7 (range 5-10) off the operating carrier's coupon value | ESTIMATE bracketed by the ISC figures |
| proration | Codeshare SPA, else MPA | [R] |
| marketing_flight_range | Four-digit block reserved per partner | [V: examples] |
| mct_rule | Operating carrier's MCT unless the marketing carrier files one | [V] |
| disclosure | "Operated by <corporate name> d/b/a <brand>" on every display, in the US | [V: 14 CFR 257] |
| needs_route_authority | true: the marketing carrier needs the traffic rights | [R] |

**AllianceMembership** (see C below for the alliance itself)

| Field | Default | Basis |
|---|---|---|
| tier | FULL, CONNECTING, AFFILIATE (regional or subsidiary of a member) | [V] |
| sponsors | 1 for full; at least 3 for a connecting tier | [V for oneworld connect and Star Connecting Partner] |
| bilateral_mesh_required | All members for FULL; sponsors only for CONNECTING | [V, secondary] |
| joining_lead_time_months | 12-18 | ESTIMATE |
| joining_cost_usd | 10-30 million for FULL; 2-5 million for CONNECTING | ESTIMATE, not sourced |
| annual_dues_usd | 1-8 million scaled to RPKs; about a quarter of that for CONNECTING | ESTIMATE, not sourced |
| exit_notice_months | 12 | [V: "at least a year to untangle"] |

**FFPPartnership** (bilateral; implied between all full members)

| Field | Default | Basis |
|---|---|---|
| accrual_price_per_mile_usd | 0.012 (range 0.008-0.015), paid by the operating carrier to the programme owner | ESTIMATE |
| partner_earn_rate | 25-100% of flown miles in discount economy, 100-150% in premium | ESTIMATE |
| redemption_reimbursement | Table by region pair and cabin: about USD 40 short-haul economy, 120 long-haul economy, 450 long-haul business | ESTIMATE |
| award_rbd | X, I, O in Star; T, U, Z in oneworld; X, O in SkyTeam | [R] |
| partner_award_release | 50-80% of what the operator offers its own members | ESTIMATE |
| elite_recognition | Tier map to the alliance tiers | [V] |
| lounge_fee_per_visit_usd | 35 (range 25-50) | ESTIMATE |

**JointVenture**

| Field | Default | Basis |
|---|---|---|
| market_scope | Region pair, for example North America-Europe | [V] |
| approvals[] | Per jurisdiction: status, date, review date (five years after grant in the US), conditions | [V: AAI] |
| requires_open_skies | true for a US grant | [V: IATA; GAO] |
| carve_outs[] | Route list with fare-type scope; empty for post-2009 JVs | [V] |
| remedies[] | Slot divestitures, SPAs and fare combinability for rivals, FFP access, data reporting, ten-year term in the EU | [V: EC] |
| pooling | REVENUE or PROFIT. Revenue: A++ and the oneworld transatlantic JV. Profit: the SkyTeam transatlantic JV | [V: EC] |
| sharing_formula | Share of pool in proportion to each partner's capacity (ASKs) in scope, or fixed percentages. A 50:50 split is the textbook case | Capacity-share rule ESTIMATE; 50:50 illustration [V: IATA] |
| settlement | Quarterly true-up payment from the partner that collected more than its share | ESTIMATE |
| capacity_band | Each partner keeps its capacity share within plus or minus 2-3 points of an agreed baseline, or the formula rebalances | ESTIMATE |
| joint_pricing, joint_rm, joint_sales | true; aligned base fares and booking classes | [V: Lufthansa deck] |
| exclusivity | false: DOT struck exclusivity clauses in Delta-Aeromexico | [V: AAI] |
| competition_effect | Partners count as one competitor on overlap routes | [V: Gillespie & Richard; Brueckner & Singer] |

**CapacityPurchaseAgreement**

| Field | Default | Basis |
|---|---|---|
| major, operator, fleet_lines[] | Aircraft type, count, in-service and removal dates | [V] |
| compensation | Per aircraft-month plus per block hour plus per departure | [V] |
| rate_per_block_hour_usd | 1,500 for a 70-76 seat jet excluding fuel and aircraft ownership; add 400-700 per block hour where the operator owns the aircraft | ESTIMATE from Mesa FY2019 (1,497) and the SkyWest comparison |
| pass_through | Fuel, landing fees, airport rents and ground handling paid by the major; insurance and property tax reimbursed at cost | [V] |
| incentives | Plus or minus 1-3% of fees on completion factor, controllable on-time departures, customer scores | Structure [V]; size ESTIMATE |
| cancellation_rule | Cancelled by the major: paid as if flown. Controllable by the operator: unpaid and penalised. Uncontrollable: fixed component only | [V: United/Republic form CPA for the first two] |
| revenue_owner, fuel_risk | Major | [V] |
| term_years | 5-12 for new aircraft; 2-5 on extension | [V: ranges in filings] |
| scope_constraint | Reference to the major's ScopeClause record | [V] |
| variant PRORATE | Operator keeps a negotiated share of the fare, pays all costs including fuel, 30-120 days' notice | [V: SkyWest] |
| variant FRANCHISE | Operator keeps all revenue and pays a brand fee of 5-10% of revenue | ESTIMATE |

**ScopeClause** (attribute of a major): max_seats 76; max_mtow_lb 86,000; caps by seat band as absolute counts (Delta, United) or as a ratio to the mainline narrowbody fleet (American). [V]

**EquityStake**: holder, target, percent, date, board_seats, linked JV. Rule: a stake of 20% or more in a carrier belonging to a rival alliance triggers an alliance exit within 6-18 months (LATAM, ITA, SAS, Asiana). [V for the cases; the rule is my generalisation]

### B. Passenger-demand effects to apply

**1. Fare level on connecting itineraries, by cooperation tier.** Multiplier on the through fare relative to a non-aligned interline connection.

| Tier | Default | 1990s scenario | Basis |
|---|---|---|---|
| Non-aligned interline | 1.00 | 1.00 | Reference |
| Codeshare only | 0.96 | 0.93 | Brueckner, Lee & Singer -3.6%; Brueckner (2003) -7% [V] |
| Alliance partners with codeshare, no immunity | 0.94 | 0.89 | -6.3%; -11% [V] |
| Immunised, not a JV | 0.90 | 0.75 | -11.2% and -6.4%; -25% to -27% [V] |
| Metal-neutral JV | 0.88 | n/a | Statistically equal to online [V] |
| Online, single carrier | 0.87 | 0.75 | -14.4% (1998-2009) and -7.4% (1997-2016) [V] |

A structural alternative gives the same result without a table: let each carrier on an interline itinerary add its own markup to its sector, and let alliance, immunity and JV status remove an increasing fraction of the second markup (0.2, 0.7, 1.0).

**2. Overlap routes.** On a nonstop route served by both partners, count immunised or JV partners as one competitor. Apply +7% to the fare for each competitor removed (range 4-15%: 15% for two to one, about 6.5% for three to two and four to three) and -10% for a low-cost carrier on the route. [V: DOJ; Gillespie & Richard; Brueckner & Singer]. A regulator object can offset this by forcing slot divestiture that brings an entrant. Carve-outs should have no fare effect and should degrade the partners' inventory decisions, as in the IATA table.

**3. Itinerary attractiveness by connection type.** Weight applied in the itinerary choice model, separate from fare.

| Path type | Weight | Basis |
|---|---|---|
| Nonstop | Calibrate so about 75% choose nonstop at an 8-10% fare premium where both exist | [V: DOJ; Gillespie & Richard] |
| Online connection | 1.00 | |
| JV partner connection | 1.00 | Aligned schedules and fares, shared lounges [V: Lufthansa deck] |
| Alliance codeshare | 0.90 | ESTIMATE |
| Non-allied codeshare | 0.80 | ESTIMATE |
| Alliance interline without codeshare | 0.60 | ESTIMATE; shown below online connections in neutral displays [V: IATA] |
| Other interline | 0.30 | ESTIMATE, consistent with interline falling from about 40% to about 10% of connecting itineraries [V] |
| Self-connect on separate tickets | 0.05 | ESTIMATE |

**4. Connection time.** Use the operating carriers' filed MCT. Apply a co-location exception for alliance members sharing a terminal (ESTIMATE: 15-30 minutes below the inter-terminal MCT). Misconnect handling follows Resolution 735d: the delivering carrier pays and rebooks.

**5. Traffic stimulation from a new partnership.** On O&Ds that become connectable under a new codeshare or JV, raise demand captured by the pair by 50-100% over two to three years (ESTIMATE). Basis: Northwest-KLM connecting passengers on Northwest aircraft roughly doubled in three years; United-Lufthansa added about 600 passengers a day in year one [V: GAO]. Hub-to-hub capacity under a JV rises 3-5% [V: via AAI].

**6. FFP preference.** For a traveller enrolled in programme P: a fare-equivalent premium of 5% for flights that earn in P at full rate, scaled by the partner earn rate for partner flights, zero otherwise; double for elite members where the flight also delivers lounge access and priority services. All ESTIMATE. I found no measured value; the only anchor is that members value points at 1-2 US cents each [V: Wikipedia].

**7. Corporate demand.** A JV or alliance corporate contract makes the partners' flights jointly eligible for the corporate discount. Model as a share shift of contracted business demand to the JV (ESTIMATE: 5-10 points on in-scope routes). DOJ noted that connecting opportunities help in winning corporate accounts [V].

**8. Regulatory state as an input.** Open skies lowers connecting fares by 6.7% in Brueckner & Singer's panel and is a precondition for US immunity. [V]

### C. The alliance as an object in the world

| Aspect | Recommendation | Basis |
|---|---|---|
| Identity | Name, founding date, founders, management company and seat, chair (rotating member CEO) | [V] |
| Governance | Admission by vote of member CEOs with an effective veto for a member whose home market the applicant contests (Starlux and Cathay Pacific). Each applicant needs a sponsor or mentor member (British Airways for Qatar Airways, Qantas for Fiji Airways) | [V for the cases] |
| Exclusivity | A carrier holds one alliance membership at a time. Bilateral codeshares and JVs with outsiders are permitted and common: Qantas with Emirates; Qantas codeshares with 8 oneworld members and 19 outsiders, Turkish with 16 Star members and 32 outsiders | [V: OAG 2023] |
| Tiers | FULL, CONNECTING, AFFILIATE, INTERMODAL | [V] |
| Obligations on a full member | Bilateral interline e-ticketing, baggage and through check-in with every member; FFP earn, burn and tier recognition; lounge access; IOSA; participation in alliance fares; brand use | Mixed [V] and [R], see 1(c) |
| What membership does not confer | Joint pricing, joint capacity planning or revenue pooling. Those need a JointVenture with regulatory approval | [V] |
| Economics | Dues and joining cost as in A. Benefits arrive through the demand effects in B and through transaction flows (FFP, lounge) that roughly net out between similar-sized members and favour carriers with large hubs | ESTIMATE |
| Internal structure | The largest member dominates: United about 21% of Star seats, American 41% of oneworld, Delta 30% of SkyTeam. Immunised JV groups form the alliance core; non-JV members get the benefits in B.3 only | [V: OAG 2022] |
| Entry events | Application, sponsor, 12-18 month joining programme, entry date. Optional connecting tier first, upgrade later (Fiji Airways: connect 2018, full 2025) | [V for Fiji] |
| Exit events | MERGER_OR_STAKE (Continental, US Airways, LATAM, SAS, ITA, Asiana); FAILURE (Ansett, Varig, Malev, Spanair, airberlin, Alitalia, Czech); SUSPENSION (Aeroflot, S7); STRATEGIC (Aer Lingus, China Southern); DISPUTE (Mexicana 2004) | [V] |
| Effects of an exit | FFP reciprocity and lounge access end on the exit date; codeshares are renegotiated one by one; award bookings on the leaver are not honoured after the date (Asiana); the alliance loses a home market (Star lost Australia with Ansett and Scandinavia with SAS) | [V: Global Traveler for Asiana] |

### D. Membership list by IATA code

```yaml
as_of: 2026-11-18
star_alliance:
  founded: 1997-05-14
  members: [A3, AC, AI, AV, AZ, BR, CA, CM, ET, LH, LO, LX, MS, NH, NZ, OS, OU, OZ, SA, SN, SQ, TG, TK, TP, UA, ZH]
  scheduled_changes:
    - {carrier: OZ, event: exit, date: 2026-12-16, reason: merger into KE on 2026-12-17}
  connecting_partners: [HO]
  intermodal: [Deutsche Bahn, OBB]
  recent: [{carrier: AZ, event: join, date: 2026-04-01}, {carrier: SK, event: exit, date: 2024-08-31}]
oneworld:
  founded: 1999-02-01
  members: [AA, AS, AT, AY, BA, CX, FJ, IB, JL, MH, QF, QR, RJ, UL, WY]
  group_additions: [{carrier: HA, via: AS, date: 2026-04-22}]
  member_elect: [{carrier: PR, expected: 2027}]
  suspended: [S7]
  recent: [{carrier: WY, event: join, date: 2025-06-30}, {carrier: FJ, event: full_member, date: 2025-04-01}]
skyteam:
  founded: 2000-06-22
  members: [AF, AM, AR, CI, DL, GA, KE, KL, KQ, ME, MF, MU, RO, SK, SV, UX, VN, VS]
  suspended: [SU]
  recent: [{carrier: SK, event: join, date: 2024-09-01}, {carrier: VS, event: join, date: 2023-03-02}, {carrier: AZ, event: exit, date: 2025-04-30}, {carrier: OK, event: exit, date: 2024-10-26}]
unaligned_majors: [EK, EY, WN, FR, U2, W6, 6E, AK, CZ, HU, LA, B6, VA, WS, EI, FI, G3, AD]
immunised_or_authorised_jvs:
  - {name: Atlantic JV (A++), parties: [UA, AC, LH, LX, OS, SN, EW], scope: North America-Europe, pooling: revenue, pending: AZ}
  - {name: oneworld transatlantic, parties: [AA, BA, IB, AY, EI], scope: North America-Europe, pooling: revenue}
  - {name: SkyTeam transatlantic, parties: [DL, AF, KL, VS], scope: North America-Europe, pooling: profit}
  - {name: UA-NH, parties: [UA, NH], scope: US-Japan and Asia}
  - {name: AA-JL, parties: [AA, JL], scope: US-Japan and Asia}
  - {name: DL-KE, parties: [DL, KE], scope: transpacific, since: 2018-05-01}
  - {name: DL-AM, parties: [DL, AM], scope: US-Mexico, status: active after 2026-08-20 court ruling}
  - {name: DL-LA, parties: [DL, LA], scope: US/Canada-South America}
  - {name: QF-EK, parties: [QF, EK], scope: Australia-Europe via Dubai, since: 2013-03-31}
  - {name: QF-AA, parties: [QF, AA], scope: US-Australasia}
```

Notes on the list: the three member arrays are [V: Wikipedia, cross-checked against 2026 trade coverage]. The Atlantic JV parties are [V: Lufthansa deck] and the oneworld JV parties including AY and EI are [V: Cranky Flier]. The unaligned list, the scope descriptions, and the DL-LA, QF-EK and QF-AA entries are [R] and should be confirmed. Regional affiliates (United Express, American Eagle, Delta Connection, Air Canada Express, QantasLink, KLM Cityhopper and so on) inherit the parent's alliance for flights under the parent's code.

### E. To verify before hard-coding

1. The interline service charge in the current IATA Passenger Services Conference resolutions (780-series) and the separate US agreement.
2. Any real codeshare commission, FFP per-mile rate and lounge charge. Airline revenue accounting staff or a redacted SEC exhibit are the likely sources.
3. Alliance dues and joining costs, and Star Alliance Services GmbH's staff and budget.
4. The DOT orders themselves: 93-1-11 (Northwest-KLM), 96-5-27 (United-Lufthansa), 2008-5-32 (SkyTeam), 2009-7-10 (Star/Continental), the 20 July 2010 oneworld order, and the Delta-Aeromexico orders of 2016 and 2025, in dockets OST-2008-0234, OST-2008-0252 and OST-2015-0070 on regulations.gov.
5. JV sharing formulas. DOT files the joint venture agreements under seal; EC commitment decisions describe structure but not percentages.
6. Hawaiian's status (member or affiliate) and whether the HA code is still marketed.
7. Current WATS alliance shares for 2023-2025.
8. The HBS Star Alliance and oneworld cases named in the brief.

---

## Sources

Primary and official
- IATA, "The Future of Interline" white paper: https://www.iata.org/contentassets/23426d4b09a0446dbe831601869098a1/future-of-interline-wp.pdf
- IATA, "Interline Considerations on Irregular Operations (IROPs): Guidance Document for Airlines", June 2020: https://www.iata.org/contentassets/e7a533819be440edbb1e49da96e0f2a8/guidance-document-interline-irops_25june2020.pdf
- IATA Clearing House: https://www.iata.org/en/services/finance/clearinghouse/
- IATA, Multilateral Interline Traffic Agreements: https://www.iata.org/en/publications/manuals/multilateral-interline-traffic-agreements/
- IATA, Prorate Manuals: https://www.iata.org/en/services/finance/prorate-manuals/
- IATA Minimum Connecting Time Technical Guide v2.2 (local copy gathered for a parallel research task)
- IATA Economics, "The Economic Benefits Generated by Alliances and Joint Ventures", 28 Nov 2011: https://www.iata.org/en/iata-repository/publications/economic-reports/benefits-of-alliances-and-jointventures
- IATA Economics, "Revisiting Alliances, Codesharing, Antitrust Immunity and International Airfares", Oct 2010 (Brueckner, Lee & Singer): https://www.iata.org/en/iata-repository/publications/economic-reports/revisiting-alliances
- US DOJ, Comments on the Show Cause Order, Star Alliance/Continental, docket OST-2008-0234, 26 June 2009: https://www.justice.gov/atr/public/comments/247556.pdf
- US DOJ, Comments, American/British Airways/Iberia/Finnair/Royal Jordanian, docket OST-2008-0252, 21 Dec 2009: https://www.justice.gov/atr/public/comments/253575.pdf
- Gillespie & Richard, "Antitrust Immunity and International Airline Alliances", DOJ EAG 11-1: https://www.justice.gov/atr/public/eag/267513.htm
- GAO/RCED-95-99, "International Aviation: Airline Alliances Produce Benefits, but Effect on Competition Is Uncertain", 6 April 1995: https://www.govinfo.gov/content/pkg/GAOREPORTS-RCED-95-99/html/GAOREPORTS-RCED-95-99.htm
- European Commission press releases IP/10/936 (14 July 2010), IP/13/456 (23 May 2013), IP/15/4966 (May 2015): https://ec.europa.eu/commission/presscorner/detail/en/IP_10_936 , https://ec.europa.eu/commission/presscorner/detail/en/IP_13_456 , https://ec.europa.eu/commission/presscorner/detail/en/IP_15_4966
- 14 CFR Part 257: https://www.law.cornell.edu/cfr/text/14/part-257
- SkyWest, Inc., Form 10-K for 2023: https://www.sec.gov/Archives/edgar/data/793733/000155837024001237/skyw-20231231x10k.htm
- Mesa Air Group, Form 10-K for fiscal 2019: https://www.sec.gov/Archives/edgar/data/810332/000156459019046104/mesa-10k_20190930.htm
- United Airlines/Republic Airways letter agreement and form capacity purchase agreement, 4 April 2025 (rates redacted): https://www.lawinsider.com/contracts/gQfM6TuNEke
- Delta, "Delta, Air France, KLM and Virgin Atlantic launch world's leading partnership", 3 Feb 2020: https://pro.delta.com/content/agency/lac/en/news/news-archive/2020/february-2020/delta--air-france--klm--and-virgin-atlantic-launch-world-s-leadi.html
- Lufthansa Group, Atlantic Joint Venture presentation: https://business.lufthansagroup.com/content/dam/b2b/about-us/joint-ventures/atlantic-joint-venture/ajv-presentation-us-small.pdf

Academic and policy
- Brueckner & Singer, "Pricing by international airline alliances: A retrospective study", Economics of Transportation 20 (2019) 100139: https://sites.socsci.uci.edu/~jkbrueck/course%20readings/DOT_study.pdf
- Brueckner & Whalen, "The Price Effects of International Airline Alliances", J. Law & Econ. 43(2), 2000, doi 10.1086/467464: https://its.uci.edu/?p=74647
- Brueckner, "International Airfares in the Age of Alliances", REStat 85(1), 2003: https://ideas.repec.org/a/tpr/restat/v85y2003i1p105-118.html
- Whalen, "A Panel Data Analysis of Code-Sharing, Antitrust Immunity, and Open Skies Treaties", Rev. Ind. Org. 30, 2007: https://ideas.repec.org/a/kap/revind/v30y2007i1p39-61.html
- Brueckner, Lee & Singer, J. Comp. Law & Econ. 7(3), 2011: https://ideas.repec.org/a/oup/jcomle/v7y2011i3p573-602..html
- Moss, "Revisiting Antitrust Immunity for International Airline Alliances", American Antitrust Institute, 2018: https://www.antitrustinstitute.org/wp-content/uploads/2018/03/AAI_Revisiting-Antitrust-Immunity_R-2.28.19.pdf

Trade press, data providers and reference
- OAG, "State of Play: Where Next for Airline Alliances and Partnerships?", 19 May 2023: https://www.oag.com/blog/where-next-airline-alliances-partnerships
- OAG, "Airline Alliances... Time to Change Partners?", 7 Dec 2022: https://www.oag.com/blog/airline-alliances
- OAG, "Airline Codeshares: An Insider's Guide", 5 June 2024: https://www.oag.com/blog/airline-codeshares-an-insiders-guide
- CAPA, "LCCs and SkyTeam member airlines continue to grow their share of global aviation", 5 Oct 2011: https://centreforaviation.com/analysis/lccs-and-skyteam-member-airlines-continue-to-grow-their-share-of-global-aviation-60007
- CAPA, "Star Alliance connecting partner model evolves as Juneyao Airlines replaces Mango as first member", 17 Oct 2016: https://centreforaviation.com/analysis/star-alliance-connecting-partner-model-evolves-as-juneyao-airlines-replaces-mango-as-first-member-307269
- Aviation Strategy, "KLM-Northwest: so much for equity cement", Nov 1997: https://aviationstrategy.aero/newsletter/Nov-1997/5/KLM-Northwest:so-much-for-equity-cement
- Cranky Flier, "United's Atlantic Joint Venture Guides How United Grows", 21 Oct 2024: https://crankyflier.com/2024/10/21/uniteds-atlantic-joint-venture-guides-how-united-grows ; "Pilot Scope is American's Secret Weapon": https://crankyflier.com/?p=37871
- DWU Consulting, "Airline codeshare and joint venture economics": https://dwuconsulting.com/dwu-ai/airline-codeshare-joint-venture-economics
- Aviatize glossary, "Interline Service Charge": https://www.aviatize.com/glossary/interline-service-charge
- View from the Wing, oneworld connect launch: https://viewfromthewing.com/new-oneworld-membership-type-launches-with-fiji-airways-as-first-member/
- AirlineGeeks, 14 Nov 2025, and Eckert Seamans aviation blog on the Delta-Aeromexico stay: https://airlinegeeks.com/2025/11/14/court-temporarily-blocks-trump-admins-order-ending-delta-aeromexico-joint-venture/ , https://www.eckertseamans.com/stay-informed/blogs/aviation/federal-circuit-court-stays-dot-order-terminating-delta-aeromexico-antitrust-immunity
- ch-aviation, "Asiana Airlines to leave Star Alliance in late 4Q26": https://www.ch-aviation.com/news/168664-asiana-airlines-to-leave-star-alliance-in-late-4q26
- Global Traveler, alliance membership shifts (2026): https://www.globaltravelerusa.com/stay-informed-on-recent-and-upcoming-shifts-in-airline-alliance-memberships/
- The Bulkhead Seat, Star Alliance 29th anniversary, 14 May 2026: https://thebulkheadseat.com/star-alliance-celebrates-29-years-as-the-worlds-largest-airline-alliance/
- IATA World Air Transport Statistics press releases for 2012, 2014, 2018 and 2020 (alliance RPK shares), for example https://www.iata.org/en/pressroom/pressroom-archive/2019-press-releases/2019-07-31-01/
- Wikipedia: Airline alliance; Star Alliance; SkyTeam; Oneworld; Codeshare agreement; Scope clause; Etihad Airways; Qatar Airways; Delta Air Lines; Korean Air; LATAM Airlines Group; ITA Airways; Emirates (airline); Frequent-flyer program; Round-the-world ticket

Named in the brief but not obtained: the HBS and other teaching cases on Star Alliance and oneworld; Oum, Park & Zhang; Bilotkach; Gayle (cited above only through other papers or from memory, marked [R]); the EC/DOT 2010 joint report "Transatlantic Airline Alliances: Competitive Issues and Regulatory Approaches" (cited through IATA's reproduction).
