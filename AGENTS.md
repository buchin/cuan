# Cuan repository guidance

Cuan is for nondeveloper users. Keep ordinary onboarding and work management in
the application UI; agents handle technical defaults and execution details.

Before adding or changing an external integration, follow
[the integration design principles](docs/product/integration-principles.md).
Prefer a maintained official provider CLI where appropriate, with a simple Cuan
UI for connection, status, retry and cancellation. Use official APIs/SDKs where
better suited; a CLI is not mandatory for every capability. Never reuse Fural or
ambient developer credentials as Cuan user credentials.
