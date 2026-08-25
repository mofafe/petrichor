# CHANGELOG.md

このファイルでは Petrichor / Iolite の主な変更履歴を記録します。

リリース前は日付ごとに変更を記録し、リリース後はバージョンと日付で管理します。

## 2026-08-22

### Changed
- 旧プロジェクト名から Iolite へ改名

## 2026-05-05

### Added
- WebRTC 音声疎通を検証するためのデバッグ UI を追加
- WebRTC 検証ページへの入口を追加

### Changed
- React と Vite の設定を追加し、Web UI 開発環境を整備
- Web UI のソース構成を整理

### Fixed
- WebSocket 切断時のログ出力を整理
- WebSocket 切断ログに関するテストを追加

### In Progress
- Signaling Server と Room 管理の安定化
- 2人通話の検証と改善

## 2026-05-02

### Added
- CHANGELOG.md を追加
- 変更履歴管理を開始

### Changed
- README.md を整備
- AGENTS.md を追加・更新
- .gitignore を改善

### In Progress
- WebSocket 基盤実装
- Signaling Server 開発
- Room 管理設計
- 2人通話を最初の目標として開発中
