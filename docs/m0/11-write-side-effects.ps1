# M0 ⑪：列出 AddChannel/UpdateChannel/DeleteChannel/UpdateOption 的副作用（纯读源码，worktree = d292f5e）
$w = Resolve-Path "$PSScriptRoot\..\..\temp\clansty-m0"
function L($f,$p) { Select-String -Path (Join-Path $w $f) -Pattern $p | % { "${f}:$($_.LineNumber): $($_.Line.Trim())" } }
L 'controller\channel.go' 'func AddChannel|validateChannel\(|BatchInsertChannels|ResetProxyClientCache|InitChannelCache|CancelChannelAffinityForce|func UpdateChannel|func DeleteChannel|channel.Update\(\)'
L 'model\channel.go' 'func BatchInsertChannels|AddAbilities|func \(channel \*Channel\) Update\(\)|UpdateAbilities|func \(channel \*Channel\) Delete\(\)'
L 'model\ability.go' 'func \(channel \*Channel\) (AddAbilities|UpdateAbilities|DeleteAbilities)'
L 'controller\option.go' 'func UpdateOption|CheckGroupRatio|ValidateConsoleSettings|model.UpdateOption'
L 'model\option.go' 'func UpdateOption|func updateOptionMap|handleConfigUpdate|InvalidatePricingCache|case "(GroupRatio|AutoGroups|UserUsableGroups|Price|USDExchangeRate|ModelRatio)"'
L 'model\pricing.go' 'func InvalidatePricingCache|time.Minute\*1'
L 'model\channel_cache.go' 'func InitChannelCache|func SyncChannelCache'
L 'main.go' 'SyncChannelCache|SyncOptions'
