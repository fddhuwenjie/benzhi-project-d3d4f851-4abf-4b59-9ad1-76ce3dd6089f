# BENZHI_README

## 项目说明
- 项目：benzhi-project-d3d4f851-4abf-4b59-9ad1-76ce3dd6089f
- 项目用途：已完整实现公共建筑导向标识放样审批工作台，覆盖勘测基线冻结、路径与标识方案、确定性规则校验、整改复验、独立走查签署、不可变安装包和摘要验证。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：wayfinding-release-gate
- 项目介绍：面向公共建筑导向标识设计团队的放样审批工作台，将现场勘测、路径建模、规则校验、冲突整改、走查复核和安装包冻结收束为一条可追溯流程。
- 项目概述：面向公共建筑导向标识设计团队的放样审批工作台，将现场勘测、路径建模、规则校验、冲突整改、走查复核和安装包冻结收束为一条可追溯流程。
- 核心工作流：设计员创建放样项目并冻结建筑勘测基线，录入目的地、路径节点和候选标识，执行可达性与信息一致性校验；存在冲突时逐项提交整改证据并复验，全部通过后由另一名复核员完成模拟走查和独立签署，系统最终冻结不可变安装放样包。
- 对外接口：Go 服务提供无需 Node 构建链的原生单页浏览器工作台和仅供该页面调用的同源 JSON 接口；页面以项目状态栏、路径与标识编辑区、规则问题清单、走查复核区和冻结包查看区完成唯一主流程。服务支持 -addr=127.0.0.1:<port>，默认监听 127.0.0.1:19081，也可读取 PORT 并绑定 127.0.0.1:<PORT>，不得默认绑定 0.0.0.0 或常见低位端口。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/wayfinding -selftest -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-d3d4f851-4abf-4b59-9ad1-76ce3dd6089f-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-d3d4f851-4abf-4b59-9ad1-76ce3dd6089f-arm64 linux/arm64

docker run -it benzhi-project-d3d4f851-4abf-4b59-9ad1-76ce3dd6089f-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/wayfinding -selftest -addr=127.0.0.1:19081`
