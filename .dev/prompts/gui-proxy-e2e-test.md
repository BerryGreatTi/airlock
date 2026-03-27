# GUI Manual E2E Test: Proxy Decryption Pipeline

이전 GUI E2E 테스트(macos-e2e-test.md #1-#20)에서 다루지 않은 프록시 복호화 관련 시나리오를 검증한다.
자동화 테스트(`make test-e2e`)는 CLI 경유 검증을 커버하므로, 여기서는 GUI 경유 동작만 테스트한다.

## 사전 조건

- `make docker-build` 완료
- `make gui-build` 완료
- 테스트 워크스페이스 준비:

```bash
mkdir -p /tmp/airlock-gui-proxy-test
cd /tmp/airlock-gui-proxy-test
airlock init
cat > .env << 'EOF'
API_KEY=sk_test_gui_12345
QUOTED_VAR="hello_world"
EOF
```

## 체크리스트

| # | 테스트 | 검증 방법 | 결과 |
|---|--------|-----------|------|
| 1 | Secrets 탭 - 따옴표 값 표시 | 워크스페이스에 위 .env 등록 후 Secrets 탭(Cmd+2) 이동. `QUOTED_VAR` 값이 `hello_world`로 표시(따옴표 없이) 확인 | |
| 2 | Encrypt All 후 ENC 패턴 확인 | "Encrypt All" 클릭 후 각 값이 `ENC[age:...]` 형태로 변경 확인 | |
| 3 | Activate 후 터미널 env 확인 | 워크스페이스 Activate -> Terminal 탭 -> `echo $API_KEY` 실행. 출력이 `ENC[age:...]`로 시작하는지 확인 (평문 `sk_test_gui_12345`가 아님) | |
| 4 | 터미널에서 curl 헤더 복호화 | Terminal에서 `curl -s https://httpbin.org/headers -H "X-Key: $API_KEY"` 실행. 응답에 `"X-Key": "sk_test_gui_12345"` 표시 확인 | |
| 5 | 터미널에서 curl 바디 복호화 | Terminal에서 `curl -s -X POST https://httpbin.org/post -H "Content-Type: application/json" -d '{"secret": "'$API_KEY'"}'` 실행. 응답 JSON `data` 필드에 평문 `sk_test_gui_12345` 포함 확인 | |
| 6 | Proxy Activity Log 복호화 기록 | Containers 탭(Cmd+3) -> Proxy Activity Log 영역에서 위 curl 요청에 대한 `decrypt` 액션 로그 표시 확인 | |
| 7 | Proxy Activity Log passthrough 기록 | Terminal에서 `curl -s https://api.anthropic.com/v1/messages` 실행 후 Proxy Activity Log에 `passthrough` 액션 로그 표시 확인 | |
| 8 | 따옴표 값 복호화 | Terminal에서 `curl -s https://httpbin.org/headers -H "X-Quoted: $QUOTED_VAR"` 실행. 응답에 `"X-Quoted": "hello_world"` 표시 확인 (따옴표 없이) | |
| 9 | CA 인증서 신뢰 (curl -v) | Terminal에서 `curl -v https://httpbin.org/get 2>&1 \| head -20` 실행. SSL 핸드셰이크 성공 확인 (`SSL certificate verify ok` 또는 에러 없음) | |
| 10 | Deactivate 후 Reactivate 복호화 유지 | Deactivate(Cmd+.) -> Activate(Cmd+R) -> Terminal에서 `echo $API_KEY` 가 여전히 `ENC[age:...]`이고, `curl -s https://httpbin.org/headers -H "X-Key: $API_KEY"` 가 `sk_test_gui_12345` 반환 확인 | |

## 정리

```bash
# GUI에서 워크스페이스 Stop and Remove, 또는:
airlock stop
rm -rf /tmp/airlock-gui-proxy-test
```

## 보고 형식

```
## GUI Proxy E2E Test Results
- Date: YYYY-MM-DD
- macOS version:
- E2E: N/10 PASS

### FAIL 항목 (있는 경우)
- #N: 설명 + 스크린샷
```
