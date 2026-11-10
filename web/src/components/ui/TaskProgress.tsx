import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { StatusDot } from './StatusDot'
import { CheckCircle2, X } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import { taskProgressModel, type TaskProgressCounts } from './task-progress-model'
export { taskProgressModel } from './task-progress-model'

/** Compact counts with an explicit breakdown; never an execution-time estimate. */
export function TaskProgress({ className, ...counts }: TaskProgressCounts & { className?: string }) {
  const { t, language } = useLanguage()
  const model = taskProgressModel(counts)
  const [open, setOpen] = useState(false), [pinned, setPinned] = useState(false)
  const [position, setPosition] = useState<{left:number;top:number;width:number}>()
  const trigger = useRef<HTMLButtonElement>(null), panel = useRef<HTMLDivElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout>>()
  const id = useId()
  const clear = () => clearTimeout(timer.current)
  const close = (restore = false) => { clear(); setOpen(false); setPinned(false); if (restore) trigger.current?.focus() }
  useEffect(() => () => clearTimeout(timer.current), [])
  useLayoutEffect(() => {
    if (!open || !trigger.current || !panel.current) return
    const rect=trigger.current.getBoundingClientRect(),width=Math.min(260,innerWidth-24),height=panel.current.getBoundingClientRect().height
    setPosition({left:Math.max(12,Math.min(rect.left,innerWidth-width-12)),top:Math.max(12,Math.min(rect.bottom+8+height>innerHeight&&rect.top>height+8?rect.top-height-8:rect.bottom+8,innerHeight-height-12)),width})
    if (pinned) panel.current.focus({preventScroll:true})
  }, [open,pinned])
  useEffect(() => {
    if (!open) return
    const outside=(event:PointerEvent)=>{if(!trigger.current?.contains(event.target as Node)&&!panel.current?.contains(event.target as Node))close()}
    const escape=(event:KeyboardEvent)=>{if(event.key==='Escape'){event.preventDefault();close(true)}}
    const scroll=(event:Event)=>{if(!(event.target instanceof Node)||!panel.current?.contains(event.target))close()}
    const focus=(event:FocusEvent)=>{if(pinned&&!trigger.current?.contains(event.target as Node)&&!panel.current?.contains(event.target as Node))close()}
    const resize=()=>close()
    document.addEventListener('pointerdown',outside);document.addEventListener('keydown',escape);document.addEventListener('focusin',focus)
    window.addEventListener('scroll',scroll,true);window.addEventListener('resize',resize)
    return()=>{document.removeEventListener('pointerdown',outside);document.removeEventListener('keydown',escape);document.removeEventListener('focusin',focus);window.removeEventListener('scroll',scroll,true);window.removeEventListener('resize',resize)}
  },[open,pinned])
  // A refreshed zero/empty scope must not leave an orphaned breakdown open.
  useEffect(()=>{if(!model)close()},[!!model])
  if (!model) return null
  const format=new Intl.NumberFormat(language)
  const label=model.complete
    ? (model.mixed ? t('All remaining work completed') : t('All tasks completed'))
    : model.pending === 0 && (counts.blocked || 0) > 0
    ? t('The result needs attention.')
    : model.total === 1 && model.pending > 0
    ? t('Task in progress')
    : model.done > 0
    ? t('{{done}} of {{total}} tasks completed').replace('{{done}}',format.format(model.done)).replace('{{total}}',format.format(model.total))
      +(model.canceled>0?' · '+t('{{count}} canceled').replace('{{count}}',format.format(model.canceled)):'')
    : t('{{count}} tasks in progress').replace('{{count}}',format.format(model.pending || model.total))
  const angle=model.fraction*Math.PI*2,x=8+4.5*Math.sin(angle),y=8-4.5*Math.cos(angle)
  const leave=()=>{if(!pinned){clear();timer.current=setTimeout(()=>close(),150)}}
  const isPulsing = !model.complete && model.pending > 0
  const isSingle = model.total === 1 && !model.complete
  return <>
    <button ref={trigger} type="button" data-task-progress-trigger data-task-progress={model.mixed?'complete-with-cancellations':model.complete?'complete':'partial'}
      aria-label={label} aria-haspopup="dialog" aria-expanded={open} aria-controls={open?id:undefined}
      className={cn('inline-flex h-11 w-11 shrink-0 items-center justify-center bg-transparent focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring',model.mixed?'text-muted-foreground':model.complete?'text-[color:var(--status-success)]':'text-[color:var(--status-working)]',className)}
      onPointerDown={event=>event.stopPropagation()} onContextMenu={event=>{event.preventDefault();event.stopPropagation()}}
      onKeyDown={event=>{event.stopPropagation();if(event.key==='Escape'){event.preventDefault();close(true)}}}
      onMouseEnter={()=>{if(matchMedia('(hover: hover) and (pointer: fine)').matches){clear();setOpen(true)}}} onMouseLeave={leave}
      onClick={event=>{event.preventDefault();event.stopPropagation();clear();if(open&&pinned)close();else{setPinned(true);setOpen(true)}}}>
      <span aria-hidden="true" className="inline-flex h-4 w-4">
        {model.complete&&!model.mixed?(
          <CheckCircle2 className="h-full w-full" strokeWidth={1.75}/>
        ):isSingle?(
          <span className="flex h-full w-full items-center justify-center"><StatusDot size="md" status={isPulsing?'working':(counts.blocked||0)>0?'warning':'neutral'} /></span>
        ):(
          <svg viewBox="0 0 16 16" className={cn("h-full w-full", isPulsing && "task-pulse-subtle animate-pulse")}>
            <circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" strokeWidth="1.5" opacity="0.65"/>
            {model.mixed?<circle cx="8" cy="8" r="4.5" fill="currentColor"/>:model.fraction>0?(
              <path d={`M 8 8 L 8 3.5 A 4.5 4.5 0 ${model.fraction>0.5?1:0} 1 ${x} ${y} Z`} fill="currentColor"/>
            ):null}
          </svg>
        )}
      </span>
    </button>
    {open&&createPortal(<div ref={panel} id={id} role="dialog" aria-label={t('Task progress')} tabIndex={-1}
      className="fixed z-[110] max-h-[70dvh] overflow-y-auto border border-border/40 bg-popover p-4 text-popover-foreground shadow-lg outline-none"
      style={{left:position?.left||12,top:position?.top||12,width:position?.width||Math.min(260,innerWidth-24),visibility:position?'visible':'hidden'}}
      onPointerDown={event=>event.stopPropagation()} onClick={event=>event.stopPropagation()} onKeyDown={event=>{event.stopPropagation();if(event.key==='Escape'){event.preventDefault();close(true)}}}
      onMouseEnter={clear} onMouseLeave={leave}>
      <div className="mb-3 pr-7"><p className="text-sm font-medium">{t('Task progress')}</p><p className="mt-1 text-xs leading-relaxed text-muted-foreground">{model.complete?t(model.mixed?'All remaining work completed':'All tasks completed'):label}</p></div>
      <button type="button" aria-label={t('Close task progress')} className="absolute right-0 top-0 inline-flex h-11 w-11 items-center justify-center text-muted-foreground hover:text-foreground focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring" onClick={()=>close(true)}><X aria-hidden="true" className="h-4 w-4"/></button>
      <dl className="space-y-2 text-sm">{model.rows.map(row=><div key={row.label} className="flex items-center justify-between gap-6"><dt className="text-muted-foreground">{t(row.label)}</dt><dd className="tabular-nums">{format.format(row.count)}</dd></div>)}</dl>
      {model.canceled>0&&<p className="mt-3 text-xs leading-relaxed text-muted-foreground">{t('Canceled tasks are excluded from progress.')}</p>}
    </div>,document.body)}
  </>
}
