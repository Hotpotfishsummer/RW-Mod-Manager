# 项目执行效率与批处理优化规范 (Execution Efficiency Guidelines)

## 1. 批处理优先，严禁串行微操作 (Batching Over Serialization)
- 涉及版本更新、多组件重构、同构修改等任务时，严禁单个文件逐步反复读写。
- 采用批处理脚本或单轮批量替换一次性搞定，控制交互轮次在 1~2 轮以内。

## 2. 合并构建与验证 (Consolidated Build & Test)
- 前端 `npm run build`、Go 单元测试 `go test ./...` 与 Wails 二进制打包 `wails build` 合并执行。
- 工具链 PATH 优先使用本地固定路径 `C:\Users\HOTPOTFISH\go\go\bin;C:\Users\HOTPOTFISH\go\bin`。
