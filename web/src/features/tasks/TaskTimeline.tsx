import { useEffect, useLayoutEffect, useMemo, useRef, useState, type RefObject } from 'react'
import { ArrowDown, CheckCircle2 } from 'lucide-react'
import type { TaskDetail } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { StatusDot } from '../../components/ui/StatusDot'
import { AccordionTimeline } from '../../components/ui/AccordionTimeline'
import { ChatMarkdown } from '../inbox/ChatMarkdown'
import { AttachmentFile } from '../inbox/Attachments'
import { workTimeline } from './task-timeline'
import { relativeTime } from './task-ui'

export function TaskTimeline({ task, scrollRef }: {task: TaskDetail; scrollRef: RefObject<HTMLElement>}) {
  const { language } = useLanguage(), id = language === 'id'
  const {events,technical} = useMemo(() => workTimeline(task,id),[task,id])
  const follow = useRef(true), previousTop = useRef(0)
  const [paused,setPaused] = useState(false)
  const active = events[events.length-1]?.id === 'current' && task.status !== 'done'
  const signature = events.map(e=>e.id + ':' + !!e.pulse).join('|')
  useEffect(() => {
    const body=scrollRef.current
    if (!body) return
    const onScroll=()=>{
      const up=body.scrollTop < previousTop.current-1
      previousTop.current=body.scrollTop
      if(body.scrollHeight-body.clientHeight-body.scrollTop<=4) follow.current=true
      else if(up) follow.current=false
      setPaused(!follow.current)
    }
    body.addEventListener('scroll',onScroll,{passive:true})
    return ()=>body.removeEventListener('scroll',onScroll)
  },[scrollRef])
  useLayoutEffect(()=>{
    if(!active||!follow.current)return
    const frame=requestAnimationFrame(()=>{
      const body=scrollRef.current
      const last=body?.querySelector<HTMLElement>('[data-work-event="current"]')
      if(body&&last){body.scrollTop=Math.max(0,last.offsetTop-body.clientHeight+last.offsetHeight+24);previousTop.current=body.scrollTop}
    })
    return ()=>cancelAnimationFrame(frame)
  },[signature,active,scrollRef])
  return <section aria-label={id?'Progres pekerjaan':'Work progress'} className="space-y-4" data-work-timeline>
    <h3 className="text-sm font-medium">{id?'Progres':'Progress'}</h3>
    {!events.length&&<p className="text-sm text-muted-foreground">{id?'Belum ada progres tercatat.':'No progress recorded yet.'}</p>}
    <AccordionTimeline label={id?'Progres pekerjaan':'Work progress'} followLatest={!paused} items={events.map(event => ({
      id: event.id,
      title: event.title || ((expanded: boolean) => expanded ? (id?'Pembaruan':'Update') : <span className="line-clamp-2 font-normal">{event.body}</span>),
      meta: event.at && <time dateTime={event.at} title={new Date(event.at).toLocaleString(language)}>{new Date(event.at).toLocaleTimeString(language,{hour:'2-digit',minute:'2-digit'})}</time>,
      indicator: event.tone==='success' ? <CheckCircle2 className="h-3.5 w-3.5 text-[color:var(--status-success)]" aria-hidden /> : <StatusDot size="md" status={event.pulse?'working':event.tone==='warning'?'warning':'neutral'} pulse={!!event.pulse} aria-hidden />,
      content: event.body || event.report || event.files?.length ? <div className="space-y-3">
        {event.body && <ChatMarkdown content={event.body} className="text-[14px] !leading-[22px]" />}
        {event.files && <div className="grid gap-2">{event.files.map(file => <AttachmentFile key={file.id} attachment={file} />)}</div>}
        {event.report && <div className="task-prose break-words"><ChatMarkdown content={event.report} /></div>}
      </div> : undefined,
    }))} />
    {active&&paused&&<div className="pointer-events-none sticky bottom-3 flex justify-end"><button type="button" className="pointer-events-auto inline-flex min-h-11 items-center gap-2 rounded-lg bg-popover px-3 text-xs text-muted-foreground shadow-sm" onClick={()=>{follow.current=true;setPaused(false);const body=scrollRef.current;if(body)body.scrollTop=body.scrollHeight}}><ArrowDown className="h-4 w-4"/>{id?'Progres terbaru':'Latest activity'}</button></div>}
    {!!technical.length&&<details className="pt-3"><summary className="flex min-h-11 cursor-pointer items-center text-xs text-muted-foreground">{id?'Rincian teknis':'Technical details'}</summary><div className="space-y-4 pt-2">{technical.map(event=><details key={event.id}><summary className="min-h-11 cursor-pointer break-words text-xs text-muted-foreground">{event.activity_type.replace(/_/g,' ')} · {relativeTime(event.created_at)}</summary><pre className="whitespace-pre-wrap break-words text-xs text-muted-foreground">{event.message}</pre></details>)}</div></details>}
  </section>
}
