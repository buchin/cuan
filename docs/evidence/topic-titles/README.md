## Contextual conversation titles

Agent topic conversations may receive a concise topic label from an optional cuan-title block in the existing normal reply. No extra provider turn is made. The metadata is removed before memory/delegation processing and never authorizes execution.

At least two user turns are required; subsequent renames require three additional turns. Simple approval/acknowledgment/greeting messages do not rename topics. Existing legacy agent rooms and customer chats are excluded. Current title is supplied to the normal response context; the model is instructed to change it only for an established topic shift, in the user's language and without private details.

An atomic database compare-and-swap changes only title, retaining room/project/topic IDs and updated_at. A durable private owner-side titles file tracks automatic labels/cadence. A label changed outside automatic naming is preserved. Malformed tracking state stops renaming. Failed replies do not change titles. Existing Inbox polling picks up the label without changing the URL.

Targeted tests cover protocol validation, metadata removal from useful replies, topic/customer binding, first-turn/cadence/replay limits, activity timestamp and manual-title preservation. Existing topic isolation and automatic recovery regression also pass with deterministic provider fixtures. No paid inference or production messages are submitted.

The model may omit a title suggestion; the existing title is then retained. This is not a guarantee that every model will identify every topic change. Initial first-message naming remains the fallback.
