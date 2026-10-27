# GitHub OAuth UI verification

Clean build aa8155b passed fixture browser checks at 360/820/1440 pixels: primary
OAuth navigation, manual token hidden under Advanced, OAuth availability retry,
start failure retry, honest unavailable state and same-task return after Advanced
connection. GitHub authorization was intercepted with a fixture page; no live
GitHub authentication or AI calls were made.

Backend callback tests cover private browser state binding, single-use exchange,
encrypted credential storage and safe task return. Real OAuth App configuration
is absent, so this is not proof of production GitHub authorization. Not deployed.
