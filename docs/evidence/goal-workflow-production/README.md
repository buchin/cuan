## Production workflow release

Production https://cuan.dojo.cc runs commit
`fb624e5af9ec0cf2b4ccae1bec6a104aa288c3ae`, tag
`goal-workflow-20261021`, schema25. Activation health and private database backup
checks passed; no migration was required.

`production.json` records read-only browser verification at 390, 820 and 1440
pixels. Existing system-message rendering, primary navigation CTA, exact assets,
composer focus, typing safety, shortcut help and global navigation passed.
Post-login writes were blocked and provider calls were zero. Screenshots remain
local because they include real conversation metadata.

Native coordinator delivery was verified on dedicated staging, not by replaying
production conversations. See ../coordinator-native/README.md. Fresh account
Google authorization completion and owner GitHub consent/private repository
continuation remain separate unverified gates. This release is not a claim that
the entire product goal is complete.
