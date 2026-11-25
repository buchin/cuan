import type { RefObject } from 'react'
import type { TaskDetail } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { ChatMarkdown } from '../inbox/ChatMarkdown'
import { TaskTimeline } from './TaskTimeline'
import { parseTaskInstructions, sanitizeCompletedTaskNote } from './task-ui'

export function TaskWorkDetails({task,scrollRef}:{task:TaskDetail;scrollRef:RefObject<HTMLElement>}) {
  const {language}=useLanguage(),id=language==='id'
  const update=task.status==='done'?sanitizeCompletedTaskNote(task.summary||'',true):task.latest_progress
  const instructions=parseTaskInstructions(task.description || task.body || task.prompt || '').cleanText
  return <div className="space-y-6">
    {!!update?.trim()&&<p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-muted-foreground">{update}</p>}
    <TaskTimeline task={task} scrollRef={scrollRef}/>
    {!!task.dod_checklist?.length&&<details><summary className="flex min-h-11 cursor-pointer items-center text-sm font-medium">{id?'Kriteria selesai':'Acceptance criteria'}</summary><ul className="space-y-3 pt-2">{task.dod_checklist.map((criterion,index)=>{
      const item=typeof criterion==='string'?{text:criterion,completed:false}:criterion
      return <li key={index} className="flex items-start gap-3 text-sm leading-relaxed"><span aria-hidden className={item.completed?'text-[color:var(--status-success)]':'text-muted-foreground'}>{item.completed?'✓':'○'}</span><div className="min-w-0 break-words"><p>{item.text}</p>{'evidence' in item&&item.evidence&&<p className="mt-1 text-xs text-muted-foreground">{String(item.evidence)}</p>}</div></li>
    })}</ul></details>}
    {!!instructions&&<details><summary className="flex min-h-11 cursor-pointer items-center text-sm font-medium">{id?'Permintaan awal':'Original request'}</summary><div className="task-prose pt-2 text-sm"><ChatMarkdown content={instructions}/></div></details>}
  </div>
}
