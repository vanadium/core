// This examples show how to create various different types of caveats.
d1 := expiryCaveat("1h")
printf("expire in 1h: %s\n", d1)

nyt := parseTime(time_RFC822Z, "12 Jan 20 17:00 -0500")
when := deadline(nyt, "1h")
d2 := deadlineCaveat(when)
printf("nyt: %s . dl; %s\n", nyt, when)

deny := denyAllCaveat()
allow := allowAllCaveat()
m1 := methodCaveat("m1")
m12 := methodCaveat("m1", "m2")
cavs := formatCaveats(deny, allow, m1, m12)
