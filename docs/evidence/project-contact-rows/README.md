# Projects and Contacts row layout

Production frontend build 5679f3c passed fixture browser checks at 360, 820 and
1440 pixels in light/dark themes. Desktop selected rows and menu areas share a
continuous background. Menus remain keyboard accessible with 44px touch targets.
Project icons and contact avatars use 36px, contact secondary text uses 12px and
row spacing is compact. Navigation and mobile overflow checks pass. No provider
calls or task launches were made.

Production f6ea560 passed read-only browser checks for both selected list rows, exact asset matching and actual rollback/reactivation. Schema25 and disabled auto-migration remain unchanged. All 37 retained application tables, 930 profile files, seven item states and eleven notification read flags match the pre-release snapshot. No production provider calls or GitHub writes were submitted by verification.
