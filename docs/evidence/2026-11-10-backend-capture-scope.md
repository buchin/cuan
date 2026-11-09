# Backend capture scope and generated evidence

Task msg_d0843350-b9f0-4682-b326-c0adaa2498e8 explicitly prohibited browser and
screenshot use, including "jangan browser/screenshot" and "tanpa modifikasi UI,
browser screenshot, atau email". Nevertheless current-outcome attempt
att_e2e46d662a2c21eb286e8150fc1408e6 bound scripts/verify-ui.sh at
01:30:55+08 and produced eight native captures at 01:33:33+08. This interval
includes preparation and capture; it is not a measured pure-browser duration.
The subsequent PM trace performed eight image reads for those unrelated UI
regression captures. These were application-generated evidence, not owner uploads.
This violated the original restriction and added irrelevant preparation/review.
The historical evidence is retained; the task's earlier no-capture claim must
not be treated as proof that every historical execution respected the restriction.

The existing capture prohibition now recognizes the actual slash/list wording
and bare browser bans. A vision-capable account does not authorize a capture.
Task execution capability classification also distinguishes server-delivered
outputs from original owner image inputs. Outputs remain available for review;
if the owner supplies an earlier delivered image in the original input message,
that explicit input still requires vision. No files or history are removed.

The full original task-scope regression failed before correction and passes
afterward. Capture binding with an unavailable source now stops at the original
prohibition, proving no recipe preparation is reached. Tests retain positive
image-reading requirements, original-owner reuse of an output, foreign/malformed
output isolation and real visual-review/capture lifecycle gates. Targeted server
groups pass (1.735s after compilation). Rollout and a compliant new native run
remain to observe; these tests do not retroactively legitimize prior captures.
