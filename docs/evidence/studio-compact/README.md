## Compact Studio composer

Production79c04c0/schema25 starts the goal input at104px (three lines), with
“What would you like done?” and an Indonesian translation. Existing auto-grow
remains capped at320px, and Get it done remains outside the input container.

Actual production checks passed at390/820/1440:104px initial height, growth for
long text, shrinking after deletion, draft retained after reload, CTA below the
input, no horizontal overflow or page errors. Post-login API writes were blocked;
no production inference or conversation submission occurred. See production.json.
