// Metadata adapter for small existing visual fixtures; scaled paging/filtering
// behavior is covered by task-page-performance-check and real PostgreSQL tests.
module.exports=function taskPageFixture(requestURL,body){
 const url=new URL(requestURL)
 if(url.pathname!=='/api/v1/tasks'||url.searchParams.get('view')!=='board'||!Array.isArray(body.tasks))return body
 const tasks=body.tasks,counts={all:tasks.length,active:0,completed:0,attention:0}
 for(const task of tasks){const status=task.status==='failed'?'blocked':task.status;counts[status]=(counts[status]||0)+1;if(status==='done')counts.completed++;else{if(['doing','queued','todo','backlog'].includes(status)||['running','queued'].includes(task.latest_attempt?.status))counts.active++;if(['review','blocked','failed','cancelled'].includes(status)||['failed','canceled','cancelled'].includes(task.latest_attempt?.status))counts.attention++}}
 return {...body,total:tasks.length,scope_total:tasks.length,counts,page:1,page_size:25}
}
