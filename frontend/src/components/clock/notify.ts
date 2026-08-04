import { Service as ClockService } from '../../../bindings/working/internal/modules/clock'

// notifyDesktop은 타이머·뽀모도로가 끝났음을 데스크톱 알림으로 알린다.
// 앱이 최소화되어 있거나 다른 창을 보고 있어도 알 수 있게 한다.
//
// 알림이 막혀 있거나 지원되지 않는 환경에서도 화면에서는 이미 소리와 문구로
// 알리고 있으므로, 실패는 조용히 넘긴다. 매번 오류를 띄우면 구간이 끝날 때마다
// 같은 경고가 반복된다.
export function notifyDesktop(title: string, body: string) {
  void ClockService.Notify(title, body).catch(() => {})
}
