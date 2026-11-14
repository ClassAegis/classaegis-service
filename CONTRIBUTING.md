<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# 贡献指南

感谢参与 ClassAegis。提交代码即表示你同意：在你有权授予许可的范围内，贡献内容按本仓库的 GNU Affero General Public License version 3（仅第 3 版，AGPL-3.0-only）发布。

## 提交前

1. 阅读 `README.md`、`SECURITY.md` 和 `CODE_OF_CONDUCT.md`。
2. 保持改动范围清晰，避免提交凭据、设备数据、安装包临时文件或第三方闭源文件。
3. 新增代码、文档和脚本应使用 AGPL-3.0 兼容许可；第三方代码必须保留原版权和许可声明，并在 `cmd/ui/build/THIRD-PARTY-NOTICES.txt` 中登记。

## 开发与验证

```powershell
go test ./...
go vet ./...
```

UI 改动还应在 `cmd/ui` 执行 `npm ci` 后运行项目已有的构建和测试命令。Windows 安装器改动需要验证中文安装路径、配置保留、进程退出和卸载流程。

## Pull Request

Pull Request 请说明：问题背景、行为变化、测试命令和已知限制。安全修复不要在公开 PR 中披露可直接利用的细节，先按 `SECURITY.md` 报告。

## 许可证

ClassAegis 项目自身采用 GNU AGPL-3.0-only。提交者应确保贡献内容不会违反第三方许可、隐私义务或他人知识产权。随项目分发的 mpv、FFmpeg、Vulkan、WebView2 及 npm/Cargo/Go 依赖继续适用各自许可证。

