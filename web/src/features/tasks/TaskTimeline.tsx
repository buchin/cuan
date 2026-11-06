import { useEffect, useLayoutEffect, useMemo, useRef, useState, type RefObject } from 'react'
import { ArrowDown } from 'lucide-react'
import type { TaskDetail } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { StatusDot } from '../../components/ui/StatusDot'
import { CollapsibleChatText } from '../inbox/CollapsibleChatText'
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
    <ol>
      {events.map((event,index)=><li key={event.id} data-work-event={event.id} className="relative pb-5 pl-6 last:pb-0">
        {index<events.length-1&&<span aria-hidden className="absolute bottom-0 left-[3px] top-3 w-px bg-border/60"/>}
        <StatusDot size="md" status={event.pulse?'working':event.tone==='warning'?'warning':event.tone==='success'?'success':'neutral'} pulse={!!event.pulse} className="absolute left-0 top-1.5" aria-hidden />
        <div className="flex items-baseline justify-between gap-3">{event.title?<p className="text-sm font-medium leading-5">{event.title}</p>:<div className="min-w-0 text-sm leading-relaxed text-muted-foreground"><CollapsibleChatText content={event.body || ''} maxLines={4} markdownClassName="text-[14px] !leading-[22px]" expandLabel={id?'Lihat laporan':'View report'}/></div>}{event.at&&<time dateTime={event.at} title={new Date(event.at).toLocaleString(language)} className="shrink-0 text-[11px] text-muted-foreground">{new Date(event.at).toLocaleTimeString(language,{hour:'2-digit',minute:'2-digit'})}</time>}</div>
        {event.title&&event.body&&<div className="mt-1 text-sm leading-relaxed text-muted-foreground"><CollapsibleChatText content={event.body} maxLines={4} markdownClassName="text-[14px] !leading-[22px]" expandLabel={id?'Lihat laporan':'View report'}/></div>}
        {event.files&&<div className="mt-2 grid gap-2">{event.files.map(file=><AttachmentFile key={file.id} attachment={file}/>)}</div>}
        {event.report&&<details className="mt-1"><summary className="flex min-h-11 cursor-pointer items-center text-xs text-muted-foreground hover:text-foreground">{id?'Lihat laporan':'View report'}</summary><div className="task-prose break-words text-sm"><ChatMarkdown content={event.report}/></div></details>}
      </li>)}
    </ol>
    {active&&paused&&<div className="pointer-events-none sticky bottom-3 flex justify-end"><button type="button" className="pointer-events-auto inline-flex min-h-11 items-center gap-2 rounded-lg bg-popover px-3 text-xs text-muted-foreground shadow-sm" onClick={()=>{follow.current=true;setPaused(false);const body=scrollRef.current;if(body)body.scrollTop=body.scrollHeight}}><ArrowDown className="h-4 w-4"/>{id?'Progres terbaru':'Latest activity'}</button></div>}
    {!!technical.length&&<details className="pt-3"><summary className="flex min-h-11 cursor-pointer items-center text-xs text-muted-foreground">{id?'Rincian teknis':'Technical details'}</summary><div className="space-y-4 pt-2">{technical.map(event=><details key={event.id}><summary className="min-h-11 cursor-pointer break-words text-xs text-muted-foreground">{event.activity_type.replace(/_/g,' ')} · {relativeTime(event.created_at)}</summary><pre className="whitespace-pre-wrap break-words text-xs text-muted-foreground">{event.message}</pre></details>)}</div></details>}
  </section>
}
