let audioContext: AudioContext | null = null

// playAlarm은 타이머가 끝났을 때 짧은 알림음을 낸다.
// 음원 파일 없이 WebAudio로 만들고, 오디오를 쓸 수 없는 환경에서는 조용히 넘어간다.
export function playAlarm(beeps = 2) {
  try {
    const Ctor = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctor) return
    if (!audioContext) audioContext = new Ctor()
    // 사용자 동작 없이 만들어진 컨텍스트는 정지 상태일 수 있다.
    void audioContext.resume()

    const start = audioContext.currentTime
    for (let i = 0; i < beeps; i++) {
      const at = start + i * 0.35
      const oscillator = audioContext.createOscillator()
      const gain = audioContext.createGain()
      oscillator.type = 'sine'
      oscillator.frequency.value = 880
      // 뚝 끊기면 잡음이 들리므로 짧게 올렸다가 내린다.
      gain.gain.setValueAtTime(0, at)
      gain.gain.linearRampToValueAtTime(0.2, at + 0.02)
      gain.gain.linearRampToValueAtTime(0, at + 0.25)
      oscillator.connect(gain).connect(audioContext.destination)
      oscillator.start(at)
      oscillator.stop(at + 0.3)
    }
  } catch {
    // 알림음 실패는 기능 자체를 막지 않는다.
  }
}
