## 路由反馈（对标 clowder-ai D9）

上次路由决策：

- 路由方式：{{.RouteMethod}}（serial / parallel / single）
- 路由目标：{{.RouteTargets}}
- 路由原因：{{.RouteReason}}
- 是否需要重路由：{{.NeedsReroute}}

如果路由失败（@mention 格式错误、目标犬不可用），系统会反馈原因。
