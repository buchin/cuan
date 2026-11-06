-- Keep every newly observed stage; inbox_requests.steps remains a bounded live
-- projection for polling and recovery. Previously discarded stages cannot be recovered.
CREATE TABLE inbox_request_steps (
 room_id UUID NOT NULL,
 request_id UUID NOT NULL,
 seq INTEGER NOT NULL CHECK (seq > 0),
 kind TEXT NOT NULL,
 text TEXT NOT NULL,
 at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY (room_id, request_id, seq),
 FOREIGN KEY (room_id, request_id) REFERENCES inbox_requests(room_id, request_id) ON DELETE CASCADE
);
INSERT INTO inbox_request_steps(room_id,request_id,seq,kind,text,at)
 SELECT room_id,request_id,(step->>'seq')::integer,step->>'kind',step->>'text',(step->>'at')::timestamptz
 FROM inbox_requests CROSS JOIN LATERAL jsonb_array_elements(steps) step;
-- Old capped histories may have lost stages. Preserve that uncertainty instead
-- of using an earlier recovery_safe marker as permission to replay unknown work.
WITH incomplete AS (
 SELECT room_id,request_id,jsonb_build_object('seq',(SELECT MAX((s->>'seq')::integer)+1 FROM jsonb_array_elements(steps) s),'kind','trace_history_incomplete','text','Earlier stages may be missing because the previous recorder reached its storage limit. Existing history is preserved.','at',NOW()) step
 FROM inbox_requests WHERE jsonb_array_length(steps)>=100
)
UPDATE inbox_requests q SET steps=q.steps || jsonb_build_array(i.step)
 FROM incomplete i WHERE q.room_id=i.room_id AND q.request_id=i.request_id;
INSERT INTO inbox_request_steps(room_id,request_id,seq,kind,text,at)
 SELECT room_id,request_id,(step->>'seq')::integer,step->>'kind',step->>'text',(step->>'at')::timestamptz
 FROM inbox_requests CROSS JOIN LATERAL jsonb_array_elements(steps) step
 WHERE step->>'kind'='trace_history_incomplete' ON CONFLICT DO NOTHING;
