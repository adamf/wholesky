# Airline distribution economics: channel costs, money flows and steering

Research date: 18 November 2026. Purpose: parameterise sales channels in an airline reservation gateway and a world airline simulator.

## How to read this file

Every number carries one of three marks.

- **[V]** verified in this session against a document I fetched or a search result that quoted it. The source is linked.
- **[R]** recalled from background knowledge and not re-verified in this session. Treat as approximate and check before relying on it.
- **ESTIMATE** my own estimate or arithmetic. The inputs are stated.

Limit on this research: the session's shared web-search budget ran out about a third of the way through. After that I could only fetch documents whose address I already knew, and several trade sites (Business Travel News, PhocusWire, Travel Weekly, The Company Dime) refused the fetch. Sections 1, 2 and 4 rest mostly on primary documents (court opinions, DOT rule, GAO, 10-Ks, annual reports). Section 3 (current channel shares), the non-US commission regimes in section 2, and the by-country BSP cycles in section 5 rest more on [R] and should be re-checked. The OTA unit economics (eDreams ODIGEO, Despegar, MakeMyTrip, Trip.com, Booking Holdings) were not verified at all.

---

## Cost of sale by channel

Amounts are what the airline pays (or forgoes) per booking unless stated. "Segment" means one flight coupon; a domestic US round trip with connections averages 2.6 to 4 segments.

| Channel | Who is paid | Basis | Typical amount | Range | Region notes | Source |
|---|---|---|---|---|---|---|
| Airline website / app | Internal IT, payment processor, search marketing | Per booking plus % of fare for card | ESTIMATE $1–3 technology plus card fee | All-in $11.75 per ticket in 2002 including overhead, advertising and card fees (down from $23.40 in 1999) | Lowest cost everywhere; LCCs build the whole model on it | GAO-03-749 [V]; modern figure ESTIMATE |
| Airline call centre | Internal labour | Per call / booking | ESTIMATE $10–25 | Often offset by a phone booking fee of $15–35 where charged [R] | Share fell from 26% of US tickets (1999) to 24% (2002) | GAO-03-749 via Edelman case [V]; cost ESTIMATE |
| Traditional agency through GDS (EDIFACT) | GDS, which passes roughly half to the agency | Per net segment | US: $3.41 (US Airways–Sabre 2011 contract floor); $2.30 weighted average for American, Delta, United at Sabre (June 2019). GDS-wide average revenue: Amadeus €5.90 (2023), €6.25 (2024) per booking | $1.35 (Southwest at Sabre) to about $7.60 (Travelport revenue per segment 2018, includes non-air) | North American fees are lower than European (Sabre 10-K). Local (home-market) bookings are cheaper than cross-border ones (Amadeus) | SDNY opinion 2017 [V]; Company Dime via search [V]; Amadeus FY2023 review [V]; Sabre FY2024/25 10-K [V] |
| Same, per ticket | GDS | Per ticket | $11 (2002 average); about $16 for a four-segment itinerary at $4 | $11–14 per ticket assumed in 2006 trade press | United: 3.3% of revenue per segment; Sabre: 2.4% of average ticket; Orbitz: about 2% | DOT final rule 2004 [V]; GAO-03-749 [V] |
| GDS incentive to agency (not paid by airline directly, but funded by the booking fee) | Agency | Per segment, volume-tiered | $1–1.50 per booking (2002–03); $1–1.70 per segment (2002) | ESTIMATE $2.5–3.1 per booking today (derived, see 1.3) | "Sometimes around 10%" of an agency's revenue | DOT 2004 [V]; Edelman [V]; D. Del. FF50 [V] |
| GDS booking with airline surcharge (DCC) | Agency/corporate pays airline | Per ticket or per fare component | Lufthansa €16 per ticket (Sept 2015); BA/Iberia £8 per fare component (Nov 2017); Air France-KLM €11 per one-way (April 2018) | Lufthansa by GDS from Sept 2022: €17.50 Amadeus, €19.50 Sabre, €23 Travelport | European network groups; waived under private-channel deals | Section 4 [V] |
| NDC direct connect / airline API | Technology vendor, own IT | Per ticket plus subscription | "Roughly a tenth" of the Sabre GDS fee (Farelogix); American: 80–90% cheaper than GDS | Build own API: about $40m plus $20–25m a year (American) | Fixed cost dominates for small carriers | D. Del. FF86, FF129, FF155 [V] |
| NDC through GDS / "wholesale model" | Airline pays agency an incentive; agency pays GDS a technology fee | Per booking | American paid agencies $2 per NDC booking from 2017 | American saved $66m a year moving OTAs to wholesale, $35m a year from OTA direct connects | Mostly US OTAs so far | DOJ proposed findings ¶175–176; D. Del. opinion ¶275 [V] |
| OTA through GDS | GDS fee, plus commission/override to OTA; traveller may pay OTA a service fee | Per segment plus % | 2002 all-in cost to airline $19.43 per ticket. Orbitz charter airlines 2003: $5.34 per ticket over $150, $2.67 below; $4 Supplier Link fee replaced the GDS fee | Non-charter airlines paid Orbitz about $10 per ticket | Expedia Group air revenue $428m in 2024, about 3% of its revenue | GAO-03-749 [V]; Expedia results via search [V] |
| TMC / corporate | GDS fee, plus back-end override, plus the corporate discount | Per segment plus % of revenue | GDS fee as above; override ESTIMATE 1–3% of flown revenue | Moving a TMC booking off GDS to direct connect raises TMC cost about $15 per booking (CWT, BCD) | Highest yield channel; hardest to move | D. Del. FF111 [V]; override ESTIMATE |
| Base commission (where still paid) | Agency | % of fare | US: 0% since March 2002. Elsewhere 0–7% [R] | US history: 10% → capped $50 (1995) → 8% (1997) → 5% (1999) → cap $20 (2001) → 0 (2002) | India 5% → 0 in Nov 2008, partly restored; China 3% → 0 by 2015–16 [R] | NCECIC 2002 [V]; section 2 |
| Consolidator / net fare | Consolidator keeps markup | Discount off published | ESTIMATE 10–30% off published fare | Wider on long-haul premium cabins | Strong in ethnic/VFR and US outbound international | [R], unverified |
| Metasearch | Metasearch site | Per click or per booking | Google Flights: free links since January 2020. Others ESTIMATE $0.30–1.50 per click or 1–2% of fare | Large airlines often pay nothing | — | Google statement Jan 2020 [V]; CPC ESTIMATE |
| Settlement (IATA BSP / ARC) | IATA / ARC | Per transaction | ESTIMATE $0.10–0.40 per ticket | — | BSP: $242bn+ a year, 400+ airlines, 59,000+ agents | IATA BSP page [V]; fee ESTIMATE |
| Card acceptance (all channels) | Acquirer, scheme, issuer | % of sale | 2.53% on cards; 1.97% blended across all payment methods (2025) | Total cost including systems and fraud 2.33% of sales, about $5 per passenger | EU consumer cards capped at 0.2%/0.3% interchange since Dec 2015 [R] | IATA release 27 Oct 2026 [V] |
| Fraud / uncollected | — | % of sale | 0.09% of sales ($0.9bn of $1.05tn, 2025) | — | — | IATA [V], ratio ESTIMATE |
| Interline partner sale | Selling carrier | % of prorated fare | Interline service charge about 9% [R] | — | — | [R], unverified |

---

## 1. Channel cost per booking

### 1.1 GDS booking fees: level and history

**Who pays whom.** The airline pays the GDS a booking fee per flight segment. The GDS keeps part and pays the rest to the agency as an incentive. The agency pays the GDS little or nothing. The traveller may pay the agency a service fee. Since 2002 in the US the airline pays the agency no base commission, only negotiated overrides. (D. Del. findings; DOJ proposed findings ¶43; SDNY 2017 opinion.) [V]

| Date | Fee | Source |
|---|---|---|
| Mid-1980s | About $1.75–1.85 per booking after the CAB rules required uniform fees | [R] |
| 1996 | $3.27 per segment, typical for a major US airline | GAO-03-749 [V] |
| 2001 | $4.28 per segment, same basis (+30.9% in five years, faster than inflation) | GAO-03-749 [V] |
| 2001 | American: 112.4m GDS segments, more than $424m in fees, so about $3.77 per segment | Edelman case, citing American's letter to the NCECIC [V]; division ESTIMATE |
| 2002 | Average $4.25 per segment, $11 per ticket | DOT final rule, 69 FR 976 [V] |
| 2002 | Sabre top participation level (Direct Connect Availability) $4.38–4.39; lowest level (Basic Booking Request) $2.12 | DOT 2004; GAO-03-749 [V] |
| 2003 | Sabre "DCA 3-year option": 10% (DOT) to 12.5% (GAO) off, frozen three years, in exchange for all published fares including web fares. Galileo Momentum: 20% off for bookings at participating agencies, who accepted lower incentives | DOT 2004; GAO-03-749 [V] |
| 2006 | Sabre and Amadeus "held firm in charging American GDS fees of more than $4 per segment" before the 2006 deals | Edelman case [V] |
| 2011 | US Airways–Sabre contract: "at least $3.41 per flight segment" | SDNY opinion, 21 March 2017 [V] |
| 2011–16 | Southwest paid Sabre $1.35 per segment. Expedia-channel fee: $0.80 plus a search fee. US Airways' accounting expert put a "reasonable profit" fee at $1.18 | Same opinion [V] |
| June 2019 | American, Delta and United paid Sabre a weighted average of about $2.30 per segment for traditional bookings | The Company Dime, reporting a court filing; seen only in a search summary [V, secondary] |
| 2022–24 | Amadeus air distribution revenue per booking: €5.42 (2022), €5.90 (2023), about €6.25 (2024: €2,945.7m on 471.2m bookings) | Amadeus FY2023 management review [V]; 2024 release [V], division ESTIMATE |
| 2023–24 | Sabre distribution revenue per booking about $5.75 (2023) and $5.95 (2024) | ESTIMATE, backed out from growth figures in the FY2024 10-K (revenue +$117m or 6% in 2024; +$434m or 27% in 2023; bookings 354.7m and 363.2m). Includes hotel and car bookings |
| 2018 | Travelport revenue per segment $7.34 (Q2), $7.61 (Q4); includes non-air and payments | Travelport releases via search [V] |

The gap between the roughly $2.30 the big three US carriers paid in 2019 and the $5.75–6.25 the GDSs average worldwide is the main regional parameter. Sabre's 10-K says that its booking fee per transaction has traditionally been lower in North America than in Europe [V]. Amadeus attributes rising revenue per booking to "a lower weight of local bookings", which confirms that home-market bookings carry a lower fee than cross-border ones [V].

**Fee schedule elements.**

- Active segment fee: charged per segment booked, at a rate set by participation level. [V]
- Cancellation: in 2003 the airline was refunded "all but 40 cents" of a cancelled segment's fee, then charged in full for the rebooked segment. GAO's example: changing the return on a four-segment, $16 itinerary costs $16.80. [V]
- Passive segments: bookings recorded in the GDS without a live sell message. Agencies use them for ticketing and back-office purposes; airlines treat them as abuse when they hold inventory. Sabre defended them in the 2004 rulemaking. [V for existence; fee levels not found]
- Churn: repeated cancel-and-rebook to hold a seat past the ticketing deadline. Each cycle costs the airline a net fee. GAO gives this as the reason OTA bookings cost less than traditional agency bookings: OTAs require payment at booking. [V]
- Ticketing fees and per-message charges exist in modern contracts but I found no public rate card.

**US Airways v. Sabre evidence.** (11 Civ. 2725, S.D.N.Y.) [V unless marked]

- The 2011 contract's "full content" provisions: No Discounts (fares in Sabre never "more expensive or less comprehensive" than in any other outlet), No Surcharge (airline may not charge Sabre subscribers a fee), No Better Benefits (same commissions and incentives as any other channel), No Direct Connects (may not induce agencies to bypass Sabre).
- US Airways estimated 40% of its revenue was booked through Sabre and another 25% through the other GDSs.
- In 2011, 94% of travel agency locations used a single GDS.
- Sabre "controls over 50% of the market".
- From 2006 through 2012 Sabre paid more than $1.2bn in incentive fees to travel agents (as stated in the opinion; the scope is not defined).
- Damages claimed without netting off incentives: $44.5m to $73.2m, using a $1.35 or $0 benchmark fee against $3.41.
- First trial (24 Oct – 20 Dec 2016): jury awarded $5,098,142, trebled to $15.3m. Vacated by the Second Circuit on 11 Sept 2019 (938 F.3d 43), which held the market two-sided.
- Retrial verdict 19 May 2022: Sabre found to have monopolised; damages $1. US Airways had sought $300m and later sought over $139m in legal costs.

**US v. Sabre / Farelogix evidence.** (D. Del., opinion 7 April 2020, 452 F. Supp. 3d 97) [V]

- Indirect channel is "about 40% of airline bookings"; 95% of indirect sales by passenger volume go through a GDS.
- Sabre has "around 50 percent" of US agency airline bookings: 54.8% of traditional agencies and 48.0% of OTAs in 2018 on the government expert's figures.
- Farelogix Open Connect costs airlines "roughly a tenth" of Sabre's fee, priced per ticket plus subscription; Sabre is priced per segment.
- American: direct connect "80–90 percent cheaper" than Sabre.
- American's documents: "$133M in annual GDS booking fee savings" between 2004 and 2009, "driven by distribution alternatives".
- Government expert: when booking fees fell under competitive pressure, agency incentives did not fall one for one. Sabre's own planning slide shows its margin shrinking as the spread between airline fees and agency incentives narrows toward an "NDC end state".

### 1.2 What the 2006 full-content agreements did

After US CRS rules lapsed in 2004 the 2003 three-year deals expired in 2006. [V unless marked]

- American reached deals with Worldspan and Galileo by July 2006. Agencies opting into "Worldspan Super Access" or "Galileo Content Continuity" got all American fares and gave up part of their incentive ($0.80 per segment at Galileo).
- On 12 July 2006 American announced a "Source Premium" of $3.50 per segment, from 1 September, on bookings through any GDS product it had not designated competitive (all of Sabre and Amadeus, and non-opt-in Worldspan and Galileo). Trade press put this at about $7 on a non-stop round trip.
- Sabre answered with the "Efficient Access Solution": opt-in agencies get full content and accept about $0.80 less incentive per segment; airlines get a discounted fee and commit to full content for at least five years. Agencies outside the programme faced the airline's $3.50.
- Sabre signed Delta for seven years (April 2006), Continental for five (May 2006), and Northwest and US Airways for five each. American agreed pricing with Sabre and Amadeus by mid-August 2006. Sabre's deal with American cut its booking fees and barred it from marketing direct connect for at least three years (Travel Weekly legal column, via search).
- Precedent: Northwest's "Shared GDS Fee" of $7.50 per round trip and $3.75 per one-way, August 2004, withdrawn two weeks later after agency and GDS reaction.
- The Edelman case says American's GDS payment under the Worldspan and Galileo programmes "would be approximately $0.60 per segment". Read that with care: it may describe the reduction, not the fee.

### 1.3 Incentives paid to agencies, and GDS margins

- Each GDS's average payments to US travel agents rose from $22.3m (1995) to $233.4m (2002). GAO-03-749. [V]
- $1 to $1.50 per booking (Sabre's comments, 2003); $1 to $1.70 per segment in 2002 (Edelman, citing DOT and a 2006 Promedia guide). [V]
- In 2002 fewer than half of agencies paid monthly GDS fees and 60% received a signing bonus (ASTA). [V]
- "Productivity pricing" (volume thresholds before incentives are paid) was in 91% of subscriber contracts in 1998 and 56% in 2002; the average threshold fell from 252 to 194 bookings. Agencies could typically place up to 30% of bookings elsewhere before losing money. DOT 2004. [V]
- Sabre 10-K: incentive consideration per booking rose in 2023, 2024 and 2025 as agencies crossed volume thresholds. [V]
- Sabre Travel Solutions "cost of revenue, excluding technology costs" (mostly incentive consideration): $895m (2022), $1,039m (2023), $1,129m (2024). Divided by distribution bookings that is about $2.96, $2.93 and $3.11 per booking, an upper bound on the incentive. ESTIMATE from [V] inputs.
- Amadeus group cost of revenue (incentive fees to travel sellers, fees to local distributors, plus smaller items): €1,099m (2022), €1,364m (2023), 25% of group revenue. If nearly all of it sits in air distribution that is up to €3.03 per booking, or 51% of air distribution revenue. Air distribution contribution margin was 46.6% in 2023 (45.2% in 2022). ESTIMATE from [V] inputs.

Working rule for the model: the agency gets 45–52% of the GDS fee; the GDS nets 48–55% before its own technology cost. ESTIMATE.

### 1.4 Airline website and call centre

GAO's 2003 survey is still the only public like-for-like comparison. Average all-in cost to the airline per ticket, including commissions, GDS fees, card fees, overhead and advertising: [V]

| Channel | 1999 | 2002 |
|---|---|---|
| Traditional travel agency | $45.93 | $30.66 |
| Online travel agency | $25.12 | $19.43 |
| Airline's own website | $23.40 | $11.75 |

Major airlines cut total distribution cost by 25.8% over those years (from an average $732.9m to $543.6m each), or 43.6% per booking. Payments to agencies fell 57% (average $370m to $159m per airline). In 2002 major US airlines spent about $7.3bn on distribution.

Modern marginal cost of a web or app booking, excluding card fees and marketing: ESTIMATE $1–3. Call centre: ESTIMATE $10–25 (six to ten minutes of agent time fully loaded). Southwest's early-2000s claim of about $1 per web booking against several dollars through an agent is [R].

### 1.5 NDC, direct connect and aggregators

Covered in the table and 1.1. Additional points:

- Under the wholesale model the airline stops paying the GDS a booking fee for OTA bookings and pays the agency directly; the agency pays the GDS for technology. Airlines "pay less for distribution". [V]
- American estimated building its own NDC API at about $40m, with $20–25m a year to run. [V]
- TMCs resist: CWT and BCD estimated a cost increase of about $15 per booking if transactions left the GDS for direct connects. [V] That is the size of the incentive plus servicing efficiency a carrier has to replace.
- Aggregator pricing (Duffel, Travelfusion and similar) is charged to the seller, typically a few dollars per order plus around 1% of value [R].

### 1.6 Metasearch

- Google said in January 2020 that it would stop charging partners for referral links on Google Flights from that month. [V]
- Skyscanner, Kayak and others: cost per click or per acquisition. ESTIMATE $0.30–1.50 per click, or 1–2% of fare on a per-booking deal. Not verified.

### 1.7 OTA economics

What an OTA earns on an air ticket, in order of size: GDS incentive; airline override or back-end commission; a service or booking fee charged to the traveller; markup on net or merchant fares; ancillaries, insurance and advertising.

- Orbitz, 2003: charter airlines paid $5.34 per ticket above $150 and $2.67 below, and got back up to $3 of the roughly $16 GDS fee. Non-charter airlines paid about $10 per ticket. Supplier Link replaced the GDS fee with $4 per ticket and carried over 70% of four airlines' Orbitz bookings. [V]
- Travellers "typically pay a $5–$10 fee" to online sites per ticket (2003). [V]
- Expedia Group air revenue: $362m (2022), $410m (2023), $428m (2024), $407m (2025), about 3% of group revenue. [V via search] Expedia no longer reports ticket counts; ESTIMATE $5–8 of revenue per ticket.
- Expedia's Sabre-channel fee benchmark in the US Airways trial: $0.80 per segment plus a search fee. [V]
- By early 2020, 10–11% of American's OTA bookings ran over its own NDC API. [V]
- Ryanair says OTAs were about 10% of its ticket sales before the pandemic and 20% by 2021 (press report). [V, secondary]
- Not verified: MakeMyTrip air ticketing margin of roughly 6–7% of gross bookings [R]; Booking Holdings airline tickets of roughly 49m in 2024 [R]; eDreams ODIGEO, Despegar and Trip.com figures. Treat an OTA air take rate of 2–4% of ticket value in mature markets and 5–7% in India and Latin America as ESTIMATE.
- Consolidators and net fares: ESTIMATE 10–30% below published; the consolidator's margin is the markup it can hold. Opaque fares (Priceline bids, Hotwire) sold distressed inventory without naming carrier or times until purchase (GAO 2003). [V for mechanism]

### 1.8 Settlement, cards, fraud

- IATA BSP: "$242B+ settled annually", 400+ airlines, 59,000+ accredited agents. [V]
- ARC: weekly settlement of US agency sales, roughly $90–100bn a year. [R]
- Card and payment cost, IATA figures for 2025: $24.5bn to process $1.05tn of sales. Payment fees $20.7bn (84.5%), airline operating costs $2.9bn, fraud or uncollected sales $0.9bn. Cards averaged 2.53%; all methods 1.97%; about $5 per passenger, against a forecast net profit of $4.50 per passenger. [V]
- UK: surcharging for consumer card payments banned from 13 January 2018 (Payment Services Regulations 2017). Before that some travel firms charged up to 3%, and Flybe 3% on credit cards and PayPal. BA stopped on 10 January 2018. [V] Since 2011–13 UK rules had already limited surcharges to cost. [V in outline]
- EU interchange caps of 0.2% debit and 0.3% credit on consumer cards from December 2015. [R]
- Chargebacks: IATA does not split them out. ESTIMATE 0.1–0.5% of card sales in normal times, far higher after mass cancellations.

---

## 2. Travel agency commission history

### 2.1 United States

| Date | Change | Source |
|---|---|---|
| Before 1978 | Regulated commission about 7% (ESTIMATE, derived: the 10% of the early 1990s was "more than 40 percent higher" than before deregulation) | NCECIC 2002 [V for the quote] |
| Early 1990s | 10% base, plus overrides for agencies meeting share or growth targets | NCECIC [V] |
| 9–10 Feb 1995 | Delta caps domestic commission at $25 one-way, $50 round trip (10% retained up to a $500 fare). Most majors match | NCECIC; Travel Weekly and BTN via search [V] |
| 1995 | TWA restores 10% after settling an antitrust case; "sales did not increase", and it reinstates the cap months later | NCECIC [V] |
| Sept 1997 | Domestic base cut from 10% to 8% (United led) | NCECIC; Travel Weekly [V] |
| Nov 1998 | International commissions capped at $100 | NCECIC [V] |
| Oct 1999 | Domestic base cut to 5% | NCECIC [V] |
| Aug 2001 | Domestic cap cut from $50 to $20 round trip ($10 one-way) | NCECIC [V] |
| 14–15 March 2002 | Delta eliminates domestic base commission; the other majors follow within days | NCECIC; BTN "Delta Moves To Zero" via search [V] |

Effects recorded by the NCECIC: [V]

- Agencies peaked at 23,937 in 1994; 18,425 at end 2001; 16,470 by mid-2002. That is 31% fewer agencies and 21% fewer locations than 1994.
- Agency air sales peaked at $83.5bn in 2000 and fell to $69.9bn in 2001.
- Commission income peaked at $6.64bn in 1997; $4.8bn in 2000; $3.6bn in 2001.
- Under 2.8% of agents charged service fees before 1995; 95% were expected to by 2002. A consultant found "relatively little demand elasticity for service fees up to $25" and expected resistance at $40.
- Delta "did not suffer any loss of travel agent business" from moving first in 1995.
- Overrides "increased since base commissions were cut to zero", concentrated in large agencies and consortia.

Longer run: about 12,000 agency locations in 1976 and 27,000 in 1986 (GAO 1986) [V]; a peak of roughly 33,000 ARC locations in the late 1990s and around 10,000–12,000 by the early 2020s [R].

Agents' antitrust suits [R]: a class action over the 1995 caps settled for about $86m in 1996–97; a later suit over the 1997–2002 cuts lost on summary judgment in 2003 and on appeal in 2004; a third was dismissed and the dismissal affirmed in 2009.

### 2.2 Elsewhere

- India: several carriers went to zero from 1 November 2008, replacing the 5% commission with a transaction fee collected from the traveller. The agents' association boycotted airlines. Jet, Kingfisher and Air India were reported to have agreed 3%; Singapore Airlines offered 1%. The regulator later directed airlines that had gone to zero to agree a commission with agents, saying commission could not be replaced by a transaction fee. [V via search and a March 2010 Travel Market Report article] A 9% commission clause was removed from IATA's Passenger Sales Agency Agreement on price-fixing grounds. [V]
- China: the big three cut agency commission in steps from 3% to zero in 2014–15 and moved to a fixed handling fee per segment; state owners pushed them toward 50% direct sales. [R]
- Europe: Lufthansa went to zero in Germany in 2004; KLM, Air France and British Airways (1%) followed in 2005. [R]
- Middle East, Africa, parts of Latin America and South-East Asia: base commission of 1–7% persists, often 5% on Gulf carriers in home markets. [R, low confidence]
- IATA framework: Resolution 824 is the standard Passenger Sales Agency Agreement. The "NewGen ISS" programme (2018–20) added three accreditation levels, a remittance holding capacity that caps an agent's unpaid cash sales, a pay-as-you-go wallet (EasyPay) and default insurance. [R]

### 2.3 Overrides and share shift

- DOT's 1988 study found CRS-owning airlines' revenues were "9 to 16 percent higher than what they would have been without CRS ownership", after display bias had been outlawed (as reported in GAO/RCED-92-130). [V]
- United's 1983 internal analysis: automating an agency with Apollo instead of a rival's system raised United's revenue from that agency by 13%; United later argued 6.6%. (GAO/RCED-86-74.) [V]
- American: the first 200 agency installations of Sabre yielded $20.1m a year of incremental passenger revenue, "a 500% return" (1977 internal memo cited in the Edelman case). [V]
- Override commissions: in a late-1980s trade survey more than half of agents said they "usually" or "sometimes" chose a carrier to earn an override. [R]
- Concentration: agencies with over $50m in revenue were under 1% of agencies and booked almost 60% of agency sales, so they received most overrides (GAO 2003). [V]

---

## 3. Channel share

### 3.1 United States

| Year | Traditional agencies | Online agencies | Airline call centres | Airline websites | Source |
|---|---|---|---|---|---|
| 1999 | 67% | 4% | 26% | 3% | GAO via Edelman [V] |
| 2002 | 46% | 17% | 24% | 13% | Same [V] |
| 2005 | — | — | — | 38% | Same [V] |

- Direct share by US passenger volume: about 50% in 2005; 58.8% in 2018. "Direct channel sales now account for about 50–70% of airline bookings." (D. Del. FF22.) [V]
- DOJ complaint, 2019: traditional agencies about 25% of US airline bookings, OTAs about 20%. [V]
- American: 50% of revenue through the indirect channel (testimony, 2020). [V]
- Network carriers in 2002: Delta 55% of revenue from traditional agencies and 10% from OTAs; America West 67% through CRSs; Alaska 56% from agencies; the five Orbitz owners 65%. [V]
- Low-cost carriers in 2002: Southwest 20% of revenue from agencies and 46% from its website; JetBlue 10% and 63%. [V]

### 3.2 Europe and elsewhere

- Lufthansa Group direct share: 30% (2015), 45% (2018), 52% in December 2018 (Skift, 14 March 2019, via search). By mid-2026, 50% of indirect bookings were NDC and 77.5% of all bookings sat outside EDIFACT (Travel Weekly Australia, via search). [V, secondary]
- Ryanair: OTAs 10% of sales pre-pandemic and 20% by 2021, so 80–90% direct. [V, secondary]
- easyJet and most European LCCs: 90%+ direct. [R]

### 3.3 Global GDS share

ESTIMATE. Amadeus processed 471.2m agency bookings in 2024 and Sabre 307.5m air bookings [V]. With Travelport at an assumed 150–250m, the three carry about 0.95–1.0bn segments against roughly 4.9bn passengers [R], near 20% of segments. On [R] inputs the 2019 figure was near 30%. The often-quoted "40–50% of bookings" is true of network carriers in North America and Europe, not of the world.

Regional mix of GDS bookings, 2023–24 [V]: Amadeus: Western Europe 28.2%, North America 27.0%, Asia-Pacific 18.4%, Middle East and Africa 13.0%, Central/Eastern/Southern Europe 7.0%, Latin America 6.5%. Sabre: North America 56%, APAC 20%, EMEA 16%, Latin America 8%.

### 3.4 Yield premium of the indirect channel

- GAO 2003: airlines "process over 60 percent of their tickets—mostly high yield business traffic—through the GDSs". [V]
- SDNY 2017: brick-and-mortar agencies' clients are "primarily corporate travelers, who are higher value customers". [V]
- ESTIMATE: if indirect is 40% of bookings and 50% of revenue (the court's industry figure and American's own), the average indirect ticket is worth 1.5 times the average direct ticket.

### 3.5 NDC share

- IATA's leaderboard: airlines committing to 20% of indirect bookings through an NDC API by December 2020; Farelogix supplied nearly half of them. [V]
- American: not verified. Company statements in 2023 put "modern retailing" channels (own channels plus NDC) near 80% of bookings. [R]
- British Airways, Air France-KLM, Qantas: no figures verified.
- Mobile share: not verified.

---

## 4. Steering fights

### 4.1 European surcharges

**Lufthansa Group.** [V unless marked]

- Announced June 2015; €16 per ticket on GDS bookings from 1 September 2015. Own website, agent portal and direct connects exempt.
- Effect on bookings: the 2015 annual report recorded a "slight adverse effect"; the chief executive later called the net financial effect "broadly neutral". A GBTA survey found 42% of 434 travel buyers said they had cut Lufthansa bookings. An executive said "small to no impact" on corporate business.
- Effect on mix: direct share from 30% to 45% in three years.
- Later levels: €19 by 2022. From 1 September 2022 by GDS: €17.50 Amadeus, €19.50 Sabre, €23 Travelport. From 5 May 2026 the Amadeus-channel charge is $22 (€19), up from $21.

**Followers.** [V via search]

- British Airways and Iberia: £8 (€9.50) per fare component from 1 November 2017, so £16 on a simple return. An increase to £9 (€10.50) was announced later. Agencies with a "private channel" agreement through Amadeus or Sabre are exempt.
- Air France-KLM: €11 per one-way from 1 April 2018. Not applied to NDC connections, its agent booking tool or direct sales. Amex GBT and Expedia negotiated exemptions.
- Emirates, Qantas, Singapore Airlines, Finnair and others introduced similar charges or channel programmes from 2019. [R]

### 4.2 American, Orbitz, Expedia and Sabre, 2010–13

- 21 December 2010: American pulls fares from Orbitz. [V]
- 1 January 2011: Expedia removes American after its contract expires on 31 December. [V]
- 5 January 2011: Sabre biases American's flights down its displays, ends a fee discount, and gives notice to terminate in August. American calls it "anti-consumer, anti-competitive". [V]
- 2011: a Texas court restrains Sabre; American returns to Expedia in April and to Orbitz in June. [R]
- October 2012: American and Sabre settle during trial. Sabre's 10-K confirms "settlement-related payment credits" to American [V]; Sabre's litigation charge for 2012 was about $347m [R].
- 2013: American settles with Travelport and Orbitz. [R]
- December 2010: Delta withdraws from a group of smaller OTAs, and in 2014–15 from a set of metasearch sites. [R]

### 4.3 Southwest

- For decades only in Sabre, at the lowest participation level, paying $1.35 per segment against $3.41 for US Airways. Agents could not change bookings. [V]
- May 2007: ten-year content agreement with Galileo. [V]
- August 2019: announces full participation in Travelport and Amadeus to win managed corporate travel. Travelport live early May 2020; Amadeus later in 2020. [V]
- 31 December 2020: full-participation agreement with Sabre; live 27 July 2021. [V]
- May 2024: fares appear on Google Flights (CBS, 23 May 2024). Kayak followed in 2024 [R]. Expedia followed in early 2025 [V for Expedia, R for date].

### 4.4 Ryanair and OTAs

- Long campaign against screen-scraping in Irish, German, Italian and US courts; a 2023 Irish High Court injunction against Flightbox. [V in part]
- Early December 2023: Booking.com, Kiwi.com and Kayak remove Ryanair flights. Ryanair confirms on 3 January 2024. [V]
- Effect: December load factor down 1 point; up to 2 points expected in January; softer yields as Ryanair "lower[s] fares where necessary" to pull bookings direct. No material effect on full-year traffic or profit guidance (up to €2.05bn at the time). Shares fell 4%. [V]
- January 2024 onward: "approved OTA" agreements, first loveholidays, then Kiwi.com, later others. Partners sell at Ryanair's price and pass customer details. [V via search]
- August 2025: agreement with Booking Holdings ends a three-year dispute. December 2025: Italy's competition authority fines Ryanair €255m for obstructing agencies. (Wikipedia.) [V, secondary]

### 4.5 American's 2023–24 NDC push

- 3 April 2023: at least 40% of fares removed from EDIFACT channels and sold only through NDC and direct. [V]
- A preferred agency programme tied benefits to NDC share; a plan to deny AAdvantage miles on bookings through non-preferred agencies was announced and then dropped. Sales support staff were cut. [V, Cranky Flier]
- ASTA filed complaints with DOT in July and September 2023. [V]
- Outcome: corporate and agency flown-revenue share fell to 11% below its historical level (Isom). Estimated 2024 revenue cost about $1.5bn. [V via search]
- Late May 2024: chief commercial officer Vasu Raja leaves. By mid-June most fares are back in EDIFACT. Basic Economy and three bundle types stay NDC-only; agents are paid commission on those bundles and on paid seats through 30 September 2024. Target: recover indirect revenue share by end of 2025. [V]

### 4.6 Regulation

- US: CAB rules 1984 (49 FR 32540) banned display bias and discriminatory fees. DOT readopted them in 1992 (57 FR 43780), tightened display rules in 1997 after Apollo's ranking was shown to disadvantage Alaska and Midwest Express, and barred enforcement of airline parity clauses in November 1997 (62 FR 59784). The final rule of 7 January 2004 ended most rules on 31 January 2004 and the rest, including the display-bias ban, on 31 July 2004. [V]
- EU: Regulation 2299/89, replaced by Regulation 80/2009 (in force March 2009), which freed fee negotiation and kept neutral display, parent-carrier obligations and limits on marketing data. [R] The Commission evaluated it in 2020; current status not checked.

### 4.7 Ownership and market share

- Sabre: separated from American on 15 March 2000; taken private March 2007 [V, Wikipedia]; partial flotation 1996 and relisting 2014 [R].
- Abacus: Sabre bought the 65% it did not own from eleven Asian airlines on 1 July 2015. [V]
- Amadeus: founded 1987 by Air France, Iberia, Lufthansa and SAS; floated 1999; buyout 2005; refloated 2010. [R]
- Galileo to Cendant 2001; Worldspan sold by its airline owners 2003; Travelport acquired Worldspan 2007; Travelport private again 2019. [R]

GDS shares, 2003 (Edelman exhibit, MetaGroup estimates) [V]:

| | USA/Canada | Central/South America | Europe/Middle East | Asia/Pacific | World |
|---|---|---|---|---|---|
| Amadeus | 9% | 38% | 49% | 15% | 26% |
| Galileo | 21% | 6% | 31% | 15% | 22% |
| Sabre | 42% | 50% | 13% | 4% | 24% |
| Worldspan | 28% | 6% | 8% | 4% | 24% |
| Abacus | | | | 19% | 4% |
| TravelSky | | | | 36% | 8% |
| Topas, Infini | | | | 4% each | 1% each |

Late 2010s, excluding TravelSky: Amadeus about 44%, Sabre about 36%, Travelport about 20%. [R] In 2024 Amadeus booked 471m against Sabre's 308m air bookings. [V] TravelSky holds China's domestic market; Russia moved to domestic systems after 2022. [R]

---

## 5. Distribution rules that matter for a simulator

### 5.1 Display order

- American's own 1981–82 analysis: agents chose the first line "more than half the time", and the chosen flight was on the first screen 92% of the time (Wikipedia's Sabre article, citing an internal memo of 3 December 1981). [V, secondary]
- Robert Crandall, 1983, on biased display: "the competitive raison d'être for having created the system in the first place" (same source). [V, secondary]
- DOT 2004: "travel agents tend to book one of the first flights displayed"; time pressure keeps them from paging. [V]
- Measured cost of an adverse ranking rule: Alaska estimated $15m a year from Apollo's mid-1990s display that ranked connections above its one-stop flights. [V]
- Revenue uplift from owning the agency's system, bias removed: 9–16% (DOT 1988). [V]
- Bias as retaliation: Sabre against American, January 2011; and US Airways said it accepted contract terms in 2006 for fear of "display biasing". [V]

### 5.2 Other rules

- Parity and full-content clauses: see 1.1. They stop an airline pricing a channel to its cost. Where they bind, the surcharge and withhold-content levers are unavailable until the contract expires. [V]
- Single-homing: 94% of US agency locations used one GDS in 2011, so leaving one GDS forfeits its agencies. [V]
- DOT's view of withdrawal: lost bookings arrive at once, fee savings later, and fees are only about 2% of the revenue at stake. [V]
- Codeshare double listing ("screen padding"): the EU code limits a codeshare flight to two carrier displays; the US never adopted a limit. [R]
- Look-to-book: API contracts cap searches per booking and charge for excess. No public rate verified.
- Agency debit memos: airlines bill agencies for churn, fictitious names, passive-segment abuse and missed ticketing deadlines. [R]
- Ticketing time limits: GDS bookings can be held unticketed until an airline-set deadline; web bookings are paid at once. US rule since 2012: 24-hour hold or free cancellation for bookings made a week or more ahead. [R]

### 5.3 Cash cycle

- BSP: agents remit cash sales on a market-specific cycle, from weekly to monthly, most often twice a month, with payment due some days after the period closes. Card sales bypass the agent: the airline is merchant and is paid by its acquirer. The by-country calendar was not verified; take it from IATA's published BSP calendars. [R]
- ARC: weekly reporting and settlement. [R]
- So an airline's cash from a cash-paid agency sale arrives roughly 5–25 days after sale depending on country; from a card sale in 1–3 days unless the acquirer holds it. ESTIMATE.
- Forward sales (air traffic liability) fund the airline; acquirers are exposed to it if the airline fails, so they demand collateral when credit weakens.
- Frontier, 2008: its processor, First Data, decided to "withhold 100% of the carrier's proceeds from ticket sales beginning May 1". Frontier filed for Chapter 11 on 10 April 2008, saying the change "would have represented a material change to our cash forecasts and business plan". (Wikipedia, quoting Frontier's release.) [V, secondary]
- Flybe (2018–19), Thomas Cook (2019), Norwegian (2019–20) and Spirit (2024) all cited acquirer collateral or holdback demands in their final months. Monarch's 2017 trigger was its licence renewal. [R]

---

## 6. Loyalty and co-brand as a revenue channel

- Delta–American Express remuneration: $6.8bn in 2023 (+22%); about $7.4bn in 2024; nearly $2bn in Q4 2024 (+14%). Long-run target $10bn. [V: Q4 figure from Delta's release; annual figures via search]
- Delta FY2024: loyalty programme other revenue $3,297m; loyalty travel awards $3,841m; total operating revenue $61,643m. [V]
- American: about $5.2bn of cash from co-brand and other partners in 2023; about $5.6bn in the twelve months to September 2024. Ten-year exclusive Citi deal announced 5 December 2024, starting 2026, expected to take payments toward $10bn a year and add about $1.5bn of pre-tax income over 2024. [V, secondary] Full-year 2024 of about $6.1bn [R].
- United: roughly $3bn a year from Chase. [R]
- Price per mile sold to banks: ESTIMATE 1–2 US cents; cost of a redeemed mile to the airline ESTIMATE 0.5–1 cent.
- 2020 financings: MileagePlus appraised at $21.9bn, against which United raised $6.8bn; SkyMiles valued about $26bn, against which Delta raised $9bn; AAdvantage appraised at $18–30bn, against which American raised $10bn in 2021. AAdvantage's reported margin was 52%. [V, secondary]

---

## Model recommendations

All values below are ESTIMATE defaults anchored on the verified points above.

### Default cost parameters

| Channel | Per segment | Per ticket | % of fare | Notes |
|---|---|---|---|---|
| Own web/app | — | $1.50 | — | Plus card fee |
| Own call centre | — | $15 | — | Optional phone fee $0–35 charged to traveller |
| GDS, North America, large carrier with full-content deal | $2.30 | — | — | Verified 2019 |
| GDS, North America, mid-size carrier | $3.40 | — | — | Verified 2011 |
| GDS, limited participation (no agency servicing) | $1.35 | — | — | Verified; cuts agency conversion |
| GDS, rest of world, cross-border booking | $6.00 | — | — | From GDS average revenue per booking |
| GDS, home-market booking outside North America | $4.00 | — | — | Local discount assumed at one third |
| GDS cancellation | $0.40 kept | — | — | 2003 figure |
| GDS share passed to agency | 45–52% of fee | — | — | Agency chooses GDS on this, not on airline cost |
| NDC direct connect | 10–20% of GDS fee | — | — | Plus fixed $20–25m a year at American's scale; scale down by carrier size |
| NDC via GDS (wholesale) | 40–60% of GDS fee | $2 incentive to agency | — | |
| OTA | Channel fee as above | — | 0–3% override | Traveller pays OTA $0–35 |
| TMC | Channel fee as above | — | 1–3% back-end | Corporate discount is separate, in pricing |
| Base commission | — | — | 0% US/EU; 1–5% elsewhere | Regulator can impose a floor (India) |
| Metasearch | — | — | 0–2% | Zero for Google |
| Settlement | — | $0.25 | — | |
| Payment | — | — | 2.0% blended; 2.5% cards; 0.5% EU consumer cards | |
| Fraud | — | — | 0.1% | |

### Channel mix by archetype (share of bookings)

| Archetype | Direct | OTA | Traditional agency / TMC |
|---|---|---|---|
| Ultra-low-cost, Europe | 85–95% | 5–15% (unauthorised unless approved) | 0–2% |
| Low-cost, US, pre-2019 Southwest style | 80–90% | 0% | 10–20% |
| US network carrier, 2019–24 | 55–65% | 15–20% | 20–25% |
| European network group after surcharge | 45–55% | 10–15% | 30–40% |
| Network carrier, Asia, Middle East, Latin America, Africa | 25–45% | 15–25% | 35–55% |
| China domestic | 40–60% | 30–40% | 10–20% |

Indirect tickets should carry about 1.5 times the average fare of direct tickets for a network carrier.

### Levers and measured response

| Lever | Measured response | Source |
|---|---|---|
| Surcharge a channel | Lufthansa, €16: direct share +15 points in three years; a "slight" booking loss; net effect "broadly neutral"; 42% of corporate buyers said they shifted some share | 4.1 |
| Surcharge, sole mover in a market with matching rivals absent | Northwest 2004 ($7.50 per round trip) and American 2006 ($3.50 per segment) both withdrew or settled within weeks. Model as a threat that buys a fee cut | 1.2 |
| Withhold content from a channel | American 2023: 40% of fares withheld; agency and corporate share fell 11%; about $1.5bn lost in a year (ESTIMATE roughly 3% of revenue); reversed after 14 months; recovery expected to take 18 months more | 4.5 |
| Leave or be dropped by OTAs | Ryanair, with 10–20% of sales through OTAs: load factor −1 to −2 points for two months, fares cut to compensate, no full-year effect. American 2010–11 returned to both OTAs within about six months [R] | 4.4, 4.2 |
| Cut base commission | US 1995–2002: no measurable loss to the first mover when rivals matched; agency count −31% in eight years; agents replaced income with service fees up to about $25 without losing customers | 2.1 |
| Pay overrides | Shifts share at large agencies, which book 60% of agency sales. More than half of agents said they shifted for overrides [R]. Suggested model: 1 point of override moves 2–5% of a contested agency's bookings (ESTIMATE) | 2.3 |
| Sign a full-content deal | Fee discount of 10–20% for three to seven years; airline gives up surcharge, web-only fares and direct-connect marketing for the term. American counted $133m a year saved 2004–09 | 1.1, 1.2 |
| Go direct-only or limited participation | Fee falls to $1.35, but corporate travel is forgone; Southwest reversed in 2019–21 to win it | 4.3 |
| Buy or lose display position | First line takes over half of agent bookings and first screen 92%. Owning the agency's system added 9–16% revenue. An adverse ranking rule cost Alaska $15m a year. A GDS can use bias as retaliation where no rule forbids it (US after July 2004) | 5.1 |
| Card acquirer holdback | Model as a step from 0% to 100% of forward card sales withheld when a credit trigger trips. This removes the air traffic liability float and can force a filing within weeks (Frontier 2008) | 5.3 |

---

## Sources

Primary documents fetched this session:

- [GAO-03-749, Airline Ticketing: Impact of Changes in the Airline Ticket Distribution Industry (July 2003)](https://www.govinfo.gov/content/pkg/GAOREPORTS-GAO-03-749/html/GAOREPORTS-GAO-03-749.htm)
- [DOT, Computer Reservations System Regulations, final rule, 69 FR 976 (7 January 2004)](https://www.govinfo.gov/content/pkg/FR-2004-01-07/html/03-32338.htm)
- [National Commission to Ensure Consumer Information and Choice in the Airline Industry, Upheaval in Travel Distribution (2002), chapter 2](https://govinfo.library.unt.edu/ncecic/reports/ch2.pdf)
- [US Airways v. Sabre, S.D.N.Y. opinion and order, 21 March 2017 (Doc. 882)](https://cases.justia.com/federal/district-courts/new-york/nysdce/1:2011cv02725/378177/882/0.pdf)
- [United States v. Sabre Corp., D. Del., opinion of 7 April 2020](https://caselaw.findlaw.com/court/us-dis-crt-d-del/2059606.html)
- [United States' proposed findings of fact, US v. Sabre (20 February 2020, redacted)](https://appliedantitrust.com/14_merger_litigation/cases_doj/sabre2019/2_d_del/sabre_ddel_pff_doj2020_02_20redacted.pdf)
- [DOJ complaint, US v. Sabre and Farelogix (August 2019)](https://www.justice.gov/opa/press-release/file/1196816/download)
- [GAO/RCED-86-74, Airline Competition: Impact of Computerized Reservation Systems (1986)](https://www.gao.gov/assets/rced-86-74.pdf)
- [GAO/RCED-92-130, Computer Reservation Systems (1992)](https://www.gao.gov/assets/rced-92-130.pdf)
- [Edelman, Distribution at American Airlines (A), 2009](https://www.benedelman.org/publications/distribution-at-american-airlines-a.pdf)
- [Amadeus FY2023 management review](https://amadeus.com/content/dam/amadeus/corporate/documents/en/investors/2023/quarterly-results/q4-2023/fy-2023-management-review.pdf)
- [Amadeus 2024 results release](https://amadeus.com/en/newsroom/press-releases/amadeus-delivers-solid-double-digit-growth-2024)
- [Sabre 10-K FY2024](https://www.sec.gov/Archives/edgar/data/1597033/000159703325000027/sabr-20241231.htm) and [FY2015](https://www.sec.gov/Archives/edgar/data/0001597033/000159703316000136/sabr-10k_20151231.htm)
- [IATA, payments cost airlines $24.5bn (27 October 2026)](https://www.iata.org/en/pressroom/2026-releases/10-27-payments-cost-airlines/)
- [IATA BSP](https://www.iata.org/en/services/finance/bsp/)
- [Delta Q4 and full-year 2024 results](https://ir.delta.com/news/news-details/2025/Delta-Air-Lines-Announces-December-Quarter-and-Full-Year-2024-Financial-Results/default.aspx)
- [Lufthansa Group, differentiation of DCC per GDS from 1 September 2022](https://business.lufthansagroup.com/de/en/news/news-dcc)

Secondary sources fetched or quoted in search results:

- [US Airways, Inc. v. Sabre Holdings Corp., No. 17-960 (2d Cir. 2019)](https://law.justia.com/cases/federal/appellate-courts/ca2/17-960/17-960-2019-09-11.html)
- [Travel Weekly, After 11 years, Sabre-US Airways case results in Sabre paying $1](https://www.travelweekly.com/Travel-News/Airline-News/Sabre-US-Airways-verdict)
- [The Company Dime, GDS fees and NDC](https://www.thecompanydime.com/gds-fees-ndc/)
- [Skift, Lufthansa now drives more than half its bookings directly (14 March 2019)](https://skift.com/2019/03/14/lufthansa-now-drives-more-than-half-its-bookings-directly)
- [Travel Market Report, Lufthansa Group to raise GDS surcharge again in May (April 2026)](https://www.travelmarketreport.com/air/articles/lufthansa-group-to-raise-gds-surcharge-again-in-may-as-ita-joins-ndc-fold)
- [Business Travel News Europe, Lufthansa and the effect of DCC](https://www.businesstravelnewseurope.com/Analysis/Lufthansa-and-the-effect-of-DCC)
- [Travel Weekly Australia, Lufthansa closes in on all-NDC future](https://travelweekly.com.au/lufthansa-closes-in-on-all-ndc-future-as-corporate-adoption-accelerates/)
- [Travolution, BA's £8 GDS surcharge to commence](https://travolution.com/articles/104771/bas-8-gds-surcharge-to-commence-from-next-week)
- [Business Travel News Europe, IAG to raise GDS surcharge](https://www.businesstravelnewseurope.com/Air-Travel/IAG-to-raise-GDS-surcharge)
- [Travel Agent Central, Expedia Group to avoid Air France-KLM GDS fee](https://www.travelagentcentral.com/running-your-business/expedia-group-to-avoid-air-france-klm-gds-fee)
- [Cranky Flier, American may have reversed course (4 June 2024)](https://crankyflier.com/2024/06/04/american-may-have-reversed-course-on-distribution-strategy-but-it-better-not-give-up-completely/) and [American begins its apology tour (17 June 2024)](https://crankyflier.com/2024/06/17/american-begins-its-apology-tour-over-sales-and-distribution-fiasco/)
- [Business Travel News, Robert Isom (2024)](https://www.businesstravelnews.com/Most-Influential/2024/Robert-Isom)
- [CAPA, Ryanair confirms removal from multiple OTA websites (4 January 2024)](https://centreforaviation.com/news/ryanair-confirms-removal-from-multiple-ota-websites-1241852)
- [Travel Extra, Ryanair shares drop 4pc as online travel agents remove airline](https://www.travelextra.ie/ryanair-shares-drop-4pc-as-online-travel-agents-remove-airline-from-website/)
- [CAPA, Sabre, Expedia drop American Airlines (January 2011)](https://centreforaviation.com/analysis/reports/sabre-expedia-drop-american-airlines-43029)
- [Skift, Southwest expands booking access and content in the GDS (19 May 2020)](https://skift.com/2020/05/19/southwest-airlines-expands-corporate-booking-gds-reach-more-business-travelers/)
- [Southwest, Sabre full participation agreement (31 December 2020)](https://www.southwestairlinesinvestorrelations.com/news-events/press-releases/detail/341/southwest-airlines-expands-channel-of-choice-signs-sabre-full-participation-gds-agreement)
- [Travel Weekly, Other GDSs likely to take cues from Sabre's AA settlement](https://www.travelweekly.com/Mark-Pestronk/Other-GDSs-likely-to-take-cues-from-Sabres-AA-settlement/)
- [Business Travel News, Delta Moves To Zero](https://www.businesstravelnews.com/More-News/Delta-Moves-To-Zero)
- [Travel Weekly, As cap anniversary nears, agents look back](https://www.travelweekly.com/Travel-News/Travel-Agent-Issues/As-cap-anniversary-nears-agents-look-back-and-ahead)
- [Travel Market Report, India airline pay reinstated but battles continue (March 2010)](https://www.travelmarketreport.com/international/articles/india-airline-pay-reinstated-but-battles-continue)
- [MediaPost, Google Flights ends charges for airline booking, referral links (January 2020)](https://www.mediapost.com/publications/article/346061/google-flights-ends-charges-for-airline-booking-r.html)
- [Which?, Rip-off card surcharges to end (January 2018)](https://www.which.co.uk/news/article/rip-off-card-surcharges-to-end-azxeM4O5lOpP)
- [Expedia Group Q4 and full-year 2024 results](https://www.nasdaq.com/press-release/expedia-group-reports-fourth-quarter-and-full-year-2024-results-2025-02-06)
- [View from the Wing, Citi's $10 billion AAdvantage deal (5 December 2024)](https://viewfromthewing.com/citis-10-billion-aadvantage-deal-how-this-windfall-shakes-up-american-airlines-and-your-miles/)
- [The Points Guy, economics of loyalty programs](https://thepointsguy.com/news/economics-of-loyalty-programs)
- [Wikipedia, Sabre (travel reservation system)](https://en.wikipedia.org/wiki/Sabre_(travel_reservation_system)); [Frontier Airlines](https://en.wikipedia.org/wiki/Frontier_Airlines); [Ryanair](https://en.wikipedia.org/wiki/Ryanair); [Southwest Airlines](https://en.wikipedia.org/wiki/Southwest_Airlines)
