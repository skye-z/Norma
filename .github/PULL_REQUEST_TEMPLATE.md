# Pull Request Checklist

- [ ] 变更目标与 `TODO.md` 对齐
- [ ] 已运行 `go test ./...`
- [ ] 已运行 `scripts/check-gofmt.ps1 -Root .`
- [ ] 已运行 `scripts/check-utf8.ps1 -Root .`
- [ ] 已运行 `scripts/check-change-gate.ps1 -Root .`
- [ ] 变更涉及文档时已同步 README/docs/TODO
- [ ] 存在高风险改动时已说明回滚方案
