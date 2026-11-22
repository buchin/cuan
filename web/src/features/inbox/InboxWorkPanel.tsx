import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'
import { ArrowLeft, X, Terminal } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useMediaQuery } from '../../hooks/useMediaQuery'
import { SurfaceHeader } from '../../components/layout/SurfaceHeader'
import { IconButton } from '../../components/ui/IconButton'
import { tasksApi, type LiveTaskSession } from '../../api/tasks'
import type { Task, TaskDetail } from '../../api/types'
import { TaskWorkDetails } from '../tasks/TaskWorkDetails'
import { isTaskActive } from './task-progress'

const TaskTerminal=lazy(()=>import('../tasks/TaskTerminal').then(module=>({default:module.TaskTerminal})))

export function InboxWorkPanel({task,view,onBack}:{task:Task;view:'details'|'terminal';onBack:()=>void}) {
  const {language}=useLanguage(),id=language==='id'
  const wide=useMediaQuery('(min-width: 1600px)'), mobile=useMediaQuery('(max-width: 1199px)')
  const [detail,setDetail]=useState<TaskDetail|null>(null),[live,setLive]=useState<LiveTaskSession|null>(null),[error,setError]=useState(''),[runtimeError,setRuntimeError]=useState('')
  const [controls,setControls]=useState<HTMLDivElement|null>(null)
  const [expandedTitle,setExpandedTitle]=useState(false)
  const title=detail?.title||task.title||(view==='terminal'?'Terminal':id?'Detail tugas':'Task details')
  const body=useRef<HTMLDivElement>(null),panel=useRef<HTMLElement>(null),mounted=useRef(true),requestVersion=useRef(0)
  const refresh=useCallback(async()=>{
    const version=++requestVersion.current
    try{const detail=await tasksApi.get(task.id);if(mounted.current&&version===requestVersion.current){setDetail(detail);setError('')}}catch{if(mounted.current&&version===requestVersion.current)setError(id?'Detail belum berhasil dimuat.':'Could not load task details.')}
    if(view==='terminal')try{const runtime=await tasksApi.runtime(task.id);if(mounted.current&&version===requestVersion.current){setLive(runtime.live_session||null);setRuntimeError('')}}catch{if(mounted.current&&version===requestVersion.current){setLive(null);setRuntimeError(id?'Koneksi terminal belum tersedia.':'Terminal connection is unavailable.')}}
  },[task.id,task.updated_at,task.review_state,task.latest_attempt?.id,view,id])
  useEffect(()=>{mounted.current=true;void refresh();return()=>{mounted.current=false}},[refresh])
  useEffect(()=>{
    if(!detail||(!isTaskActive(detail)&&!detail.review_state))return
    const interval=setInterval(()=>{
      if(typeof document !== 'undefined' && document.visibilityState !== 'visible') return
      void refresh()
    },5000)
    const onResume = () => {
      if(typeof document !== 'undefined' && document.visibilityState === 'visible') void refresh()
    }
    document.addEventListener('visibilitychange', onResume)
    return()=>{
      clearInterval(interval)
      document.removeEventListener('visibilitychange', onResume)
    }
  },[detail?.status,detail?.latest_attempt?.status,detail?.launch?.state,detail?.review_state,refresh])
  useEffect(()=>{
    const trigger=document.activeElement instanceof HTMLElement?document.activeElement:null
    panel.current?.querySelector<HTMLButtonElement>('[data-work-back]')?.focus()
    return()=>{if(trigger?.isConnected)trigger.focus({preventScroll:true})}
  },[])
  return <aside ref={panel} aria-label={view==='terminal'?'Terminal':id?'Detail tugas':'Task details'} role={mobile?'dialog':undefined} aria-modal={mobile||undefined} data-testid="inbox-work-panel" data-work-view={view}
    onKeyDown={event=>{
      if(event.key==='Escape'){event.preventDefault();onBack()}
      if(mobile&&event.key==='Tab'){
        const focusable=Array.from(event.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled),a[href],textarea,input,[tabindex="0"],summary')).filter(node=>node.getClientRects().length)
        const first=focusable[0],last=focusable[focusable.length-1]
        if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus()}
        else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus()}
      }
    }}
    className="fixed inset-0 z-50 flex min-h-0 min-w-0 flex-col bg-background min-[1200px]:relative min-[1200px]:inset-auto min-[1200px]:z-auto min-[1200px]:h-full min-[1200px]:flex-1 min-[1600px]:w-[clamp(520px,38vw,760px)] min-[1600px]:flex-none min-[1600px]:border-l">
    <SurfaceHeader title={<button type="button" data-work-title title={title} aria-expanded={expandedTitle} onClick={()=>setExpandedTitle(value=>!value)} className={`min-h-11 w-full whitespace-normal text-left leading-5 ${expandedTitle?'':'line-clamp-2'}`}>{title}</button>} leading={<div className="flex items-center gap-2">{!wide&&<IconButton data-work-back label={id?'Kembali ke percakapan':'Back to conversation'} onClick={onBack}><ArrowLeft/></IconButton>}{view==='terminal'&&<Terminal aria-hidden className="h-4 w-4 text-muted-foreground"/>}</div>} actions={wide&&<IconButton data-work-back label={id?'Tutup detail':'Close details'} onClick={onBack}><X/></IconButton>}/>
    {error&&<p role="alert" className="px-4 py-3 text-sm text-muted-foreground">{error}<button className="ml-2 min-h-11 underline" onClick={()=>void refresh()}>{id?'Coba lagi':'Try again'}</button></p>}
    {view==='terminal'?<>
      <div ref={setControls} className="flex shrink-0 flex-wrap items-center gap-1 border-b px-3"/>
      {runtimeError&&<p role="status" className="px-4 py-2 text-xs text-muted-foreground">{runtimeError}</p>}
      <div className="min-h-0 flex-1 overflow-hidden" data-work-terminal>
        {detail?.latest_attempt?<Suspense fallback={<p role="status" className="p-4 text-sm text-muted-foreground">{id?'Menghubungkan terminal…':'Connecting terminal…'}</p>}><TaskTerminal taskId={detail.id} attempt={detail.latest_attempt} live={live?.attempt_id===detail.latest_attempt.id?live:null} controlsTarget={controls} onRefresh={()=>void refresh()}/></Suspense>:<p role="status" className="p-4 text-sm text-muted-foreground">{detail?(id?'Belum ada sesi terminal.':'No terminal session yet.'):(id?'Memuat…':'Loading…')}</p>}
      </div>
    </>:<div ref={body} className="relative min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-6" data-work-scroll>
      {detail?<TaskWorkDetails task={detail} scrollRef={body}/>:!error&&<p role="status" className="text-sm text-muted-foreground">{id?'Memuat…':'Loading…'}</p>}
    </div>}
  </aside>
}
