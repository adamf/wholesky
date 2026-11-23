# Known defects

This list is from a review of the simulator's logic on 22 November 2026.
5 Claude sessions each read 1 area of the code and traced what happens
when a day does not go to plan. The review looked for places where the
world keeps, counts or charges for something that did not happen.

The first part lists what was fixed. The second part lists what is open.
An open item is marked **reported**: a reviewing session traced it in the
code, and nobody has reproduced it yet. Each fixed item was read in the
code a second time before the fix, and has a test that fails without it.
The repository's owner found the faults in 6 of the fixed items on
reading the work: 3 in the first fixes of this review, and 3 in the
overbooking and the extras.

The counts of flights are from the 2 bundled worlds: `world.json` has
103,688 flights and `thanksgiving.json` has 22,889.

## Fixed

- **The day's books closed on the next day.** The simulator sees
  midnight on the first tick after it. The close then scored each
  carrier at 00:01, when no flight of the day had flown. Each day went
  into the book with all its revenue and almost none of its cost. The
  close now scores the day at its end.
- **A flight that landed after midnight never landed.** 5,182 flights in
  `world.json` and 3,235 in `thanksgiving.json` land after midnight.
  Their departure control records stayed closed, so from the second day
  their name lists were refused and they had no check-in and no door
  count. A flight that its delay pushed past midnight never left. The
  day's loop now keeps each flight between its departure and its
  arrival, and carries it through midnight with the fate it had.
- **The last minutes of a day and its first minute were dropped.** A
  tick that crossed midnight lost the events before it, and an event at
  00:00 could never run.
- **A clock that stepped back was taken for midnight.** A region whose
  clock a peer had set back closed its day in the afternoon and began
  another.
- **A cancellation decided inside 2 hours was never announced.** The
  only announcement was at the tick 2 hours before departure. A crew
  that timed out later, or a world that started later, left the flight
  with its flight plan, its checked-in passengers and its slots. Every
  cancelled flight that has not left is now announced.
- **A closed airport cancelled flights that had flown.** The closure
  cancelled every flight of the day that touched the airport. It did not
  change the day's plan, so the flights after the reopening departed
  although their cancellations had gone out, and no slot was released.
  A closure now cancels what has not left, through the plan.
- **A cancelled flight stayed on sale.** The carrier confirmed seats on
  it until its counter would have closed, and travellers were offered
  it. A cancellation now closes every class, and the flight is for sale
  again the next day.
- **A cancellation did not end with its day.** A cancellation ended at
  the hour the flight would have landed, taken as a minute of the day.
  For a flight due in after midnight, that minute came before the
  cancellation and never after it. At a closed airport it was passed
  over. The flight stayed cancelled and closed for sale all the next
  day, with no check-in. A cancellation and its stop-sell now end at
  midnight. An announcement that is on its way at midnight goes out
  first, and one that starts after midnight reads the new day's plan.
- **A landing reopened a cancelled flight.** A flight that lands in the
  morning left the day before. Its landing took today's cancellation of
  the same number with it, and today's flight went back on sale. A
  landing now releases only the departure that landed.
- **A closure's cancellations stood on later days, or on none.** A world
  with no crew model kept the plan of the day before. A flight that a
  closure or a seat cancelled stayed cancelled each day after. An
  airport still closed at midnight left the new day's flights neither
  flown nor cancelled. The plan is now worked out again each day, and a
  closed airport cancels each day's flights. A flight that comes due at
  a closed airport before its cancellation has gone out is cancelled at
  that minute.
- **Reprotection offered flights that could not be boarded.** The
  alternatives for a stranded passenger included flights that the day
  had cancelled, flights at a closed airport and flights that had left.
- **A departure's state outlived the departure.** The world flies the
  same date each day, so each flight has the same key each day. A flight
  retimed once was never retimed again. A smaller aircraft stayed on a
  flight for good. The cancel mark and the door count were by flight
  number, so 1 leg of a number cancelled its other legs.
- **A short connection was never checked in.** A connecting party with
  under 56 minutes in hand was due at the counter after its last wave.
  It closed as a no-show and went into the record of who came. A party
  off a cancelled inbound flight still arrived. A party that an early
  wave judged to miss the flight, and that then made it, was counted as
  missed and as boarded.
- **An infant on a lap took a seat.** jetway's departure control seated
  each name of a party. A full cabin then had no seat for its last
  passenger, who was denied boarding on a flight that was not oversold.
  About 3 % of filled records have an infant. Fixed in jetway v0.1.108.
- **An overbooked cabin was full when its seats were sold.** The
  programme that prices the last seat of each leg counted seats, and not
  the bookings the cabin may hold. A cabin with room for more bookings
  had none in the programme, which priced it at the dearest fare on the
  leg. A connecting passenger had to pay more than that fare for a seat
  that the carrier wanted to sell. The programme now counts the bookings
  the cabin may hold.
- **An infant on a lap was a volunteer, a denied boarding and a
  customer.** The carrier asked each accepted passenger to give up a
  seat, so 1 infant in 25 at an oversold flight took the offer and was
  paid. An infant whose adult was turned away was counted and paid as a
  denied boarding. An infant bought seats, cabin bags and drinks. An
  infant is now asked nothing, is paid nothing and buys nothing. It
  comes off with its adult, and it is not in the counts of seats or of
  passengers carried. Fixed in jetway v0.1.109 and here.
- **A class sold only as a round trip paid no compensation.** The fare
  for a denied boarding was looked up among the one-way fares. A class
  with none gave $0. A volunteer in it was offered nothing, and a
  passenger turned away was owed a meal. The fare is now that of 1
  direction of the round trip.
- **Ticket stock codes did not name the carrier.** The seats sold before
  the day were ticketed on codes from 1 sum and the day's sales on codes
  from another, and 2 carriers could share a code. Settlement and
  interline billing find a ticket's carrier by its code. Each carrier
  now has its own code, and both kinds of sale use it.
- **A booking made on the day was dropped before its flight left.** Such
  a booking has a date and no time on its segment. Retention took its
  departure as 00:00 and removed it from the carrier's book at 03:00, so
  the passenger was on no name list. Retention now takes the time from
  the schedule.
- **Earlier the same day.** The name list carried waitlisted passengers.
  A booked passenger at a full cabin was refused and reported as a
  no-show. An aircraft change gave 2 passengers 1 seat. The scorecard
  forgot a flight's passenger count when the flight landed.

## Open: the day and its clock

- **Departures before 03:00.** Their ground story starts the evening
  before. Midnight then purges the book and writes the new day's, in the
  middle of their check-in. 3,623 flights in `thanksgiving.json` leave
  before 03:00. Reported.
- **A restart does not give back the same day.** Crews start again at
  day 0. Cancellations that were announced are lost. A midnight that
  passes while the process is down is skipped. The selling date of a
  synthetic world is the moment of start plus 30 days, with its hour.
  Reported.
- **The first minute of a day has the books of the day before.** At
  midnight the new day's bookings are written, and that takes about 1
  minute of real time. Until it is done, the inventory holds the sold
  counts of the day before under the limits of the new day. The
  invariants check then reported 1 cabin as sold 1 past its limit, on 1
  flown day of 2. Traced once.
- **A seat's answer is applied after its moment.** A seat asked about a
  crew answers in real time. At a warp of 60 the cancellation has been
  announced before the answer comes, and the answer still revives the
  flight. Reported.
- **The sortation system, the name list cache and the datalink match by
  flight number.** On a number with several legs, 1 leg's landing
  removes the other leg's bag tags, and an aircraft report can land on
  the wrong leg. 3,723 flight numbers in `thanksgiving.json` fly more
  than 1 leg. `world.json` has none. Reported by 3 reviewers.

## Open: cancellations and recovery

- **A cancelled leg does not stop the aircraft's later legs.** The
  aircraft is still where the cancelled leg began, and its next leg
  leaves from another airport on time. Only 17 % of consecutive legs are
  an out and back. The fix needs a decision: cancel down the rotation,
  or assume a spare aircraft. Reported.
- **Only bookings that a distribution system holds are reprotected.**
  The seats sold before the day are in the carrier's book alone. Nobody
  moves those passengers, and the cancelled flight keeps its sold count.
  Reported by 2 reviewers.
- **A new plan has no present.** Planning a tail again staffs its crews
  from the first leg. A crew cancellation that was announced can come
  back to life, and a flight in the air can become cancelled. Reported.
- **A reserve crew's 90 minutes delay 1 leg.** The legs after it keep
  their times. Reported.
- **The ground timetable follows the schedule and not the delay.** A
  flight 2 hours late closes its door at its scheduled time. A cancel
  after that leaves it closed with its passengers boarded. Reported by 2
  reviewers.
- **The operations desk prices a cancellation differently from the
  scorecard.** Its model has no term for the fuel and charges that a
  cancelled flight saves, and it plays the whole day from 00:00.
  Reported.
- **A flight that is late for its own reasons keeps the slot of its
  scheduled time.** Reported, and arguable.
- **A carrier on its own node.** A flight that the plan cancels still
  departs, and keeps its slot. Reported.
- **A duty that is wholly cancelled is paid as flown.** Reported.

## Open: passengers and bags

- **Connections are drawn for each leg and not read from the record.** A
  no-show on the first leg boards the second. Reported.
- **Volunteers and denials go by passenger and not by party.** Reported.
- **Rush bags can go on a cancelled flight.** The bag office's files
  outlive the day's counts. Reported.

## Open: selling

- **An infant booked on the day is not known to the airport.** A
  booking request can mark a passenger as an infant, and the mark does
  not reach the carrier. The airport then gives the infant a seat. An
  agent's own service request for the infant names no passenger, and
  the airport takes each name of the party for an infant. The day's own
  sales book no infants, so only a booking made by hand meets this.
  Traced with a test.
- **A codeshare seat is confirmed before the operator answers.** The
  marketing carrier sells from its own count, and the operator's refusal
  is never read. Reported.
- **A waitlist never clears.** A cancelled seat goes to the next request
  and not to the waitlist. A waitlisted sale is counted as sold.
  Reported.
- **Sales use scheduled times.** A connection onto a retimed flight is
  still sold. Reported.
- **A new service can take another service's flight numbers** at a later
  restart, and with them its booking record. Reported.
- **The pricing desks forget their open fights at a restart.** Reported.
- **A cut that is taken back is read as a fare increase.** A carrier
  cuts a fare, no rival answers, and the carrier goes back. The desks
  record the going back as the lead of an increase. The carrier's next
  cut in that market is read as that increase withdrawn, and no rival
  answers it. Traced in a test that failed 1 run in 10.

## Open: money

- **A cancelled flight keeps its fares.** Revenue is counted from 00:00
  for flights that have not flown. A cancellation refunds a flat 25 %.
  The fix is to count a leg's revenue when it flies. Reported.
- **The sales ledger is not a day's on a split world.** A distribution
  machine never resets its ledger, and a region reads the running total.
  A round trip sold yesterday is counted again today. Reported by 2
  reviewers.
- **Interline billing bills every ticketed coupon**, flown or not.
  Reported.
- **Reprotection onto another carrier moves no money.** Reported.
- **An agency's own fee is counted as the carrier's revenue.** Reported.
- **A reissue adds the fare to the settlement's gross again.** Reported.
- **Settlement is worked out again from books that retention is
  emptying.** Reported.

## Open: a world on several machines

- **Nothing on the core runs a day.** Fares are never filed and the
  calendar never moves. Reported.
- **A failed poll adds a peer's whole total again**, and the invariants
  check passes when a machine is down. Reported.
