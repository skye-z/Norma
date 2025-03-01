# Pull Request Checklist

- [ ] 变更目标已与 `TODO.md` 对齐
- [ ] 已运行 `go test ./...`
- [ ] 已运行 `scripts/check-gofmt.ps1 -Root .`
- [ ] 已运行 `scripts/check-utf8.ps1 -Root .`
- [ ] 已运行 `scripts/check-change-gate.ps1 -Root .`
- [ ] 文档同步项已确认：`README.md` / `docs/*` / `TODO.md` / `USE.md`
- [ ] 如涉及发布或安装流程，已同步平台验收清单与安装说明
- [ ] 如存在高风险改动，已说明回滚方案
