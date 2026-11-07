# M0 ③：PUT 写零值（priority=0、models=""）的行为
. "$PSScriptRoot\_lib.ps1"
$name = (Get-Content "$Env\ch1name.txt").Trim()
$ch = (FindChannel $name).items[0]; $id = $ch.id
function Show($t) { $c = (Api GET "/api/channel/$id").data; "$t -> priority=$($c.priority) weight=$($c.weight) models=[$($c.models)] status=$($c.status) group=$($c.group)" }
Show '初始'
"PUT priority=0:  " + (Api PUT '/api/channel/' @{id=$id; priority=0} | ConvertTo-Json -Compress -Depth 3 | % { $_ -replace '"key":"[^"]*"','"key":"***"' })
Show 'priority=0 之后'
"PUT models='':   " + (Api PUT '/api/channel/' @{id=$id; models=''} | ConvertTo-Json -Compress -Depth 3 | % { $_ -replace '"key":"[^"]*"','"key":"***"' })
Show "models='' 之后"
"PUT weight=0:    " + (Api PUT '/api/channel/' @{id=$id; weight=0}).success
Show 'weight=0 之后'
"PUT status=0:    " + (Api PUT '/api/channel/' @{id=$id; status=0}).success
Show 'status=0 之后'
"PUT status=2 / 1:" 
Api PUT '/api/channel/' @{id=$id; status=2} | Out-Null; Show 'status=2 之后'
Api PUT '/api/channel/' @{id=$id; status=1} | Out-Null; Show 'status=1 之后'
"PUT priority=5 (非零对照): " + (Api PUT '/api/channel/' @{id=$id; priority=5}).success; Show 'priority=5 之后'
"--- 恢复"
Api PUT '/api/channel/' @{id=$id; priority=3; weight=7} | Out-Null; Show '恢复后'
