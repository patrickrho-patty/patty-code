package recovery

const (
	MaxOperationFailures = 3
	MaxEpisodeFailures = 6
	MaxReviewRejects = 3
	MaxStoppedOperationRetries = 3
)

type StopReason string

const (
	StopReasonNone              StopReason = ""
	StopReasonEpisodeFailures   StopReason = "episode_failures"
	StopReasonReviewRejects     StopReason = "review_rejects"
	StopReasonStoppedOpRetries  StopReason = "stopped_op_retries"
	StopReasonOperationFailures StopReason = "operation_failures"
)

const (
	PauseMessageEN = "Automatic retries paused. Patty Code stopped repeated attempts and kept completed work. Send \"continue\" to start a fresh attempt, or add instructions to change direction."
	PauseMessageZH = "자동 재시도가 일시 중지되었습니다. Patty Code가 반복 시도를 멈추고 완료된 작업을 유지합니다. '계속' 전송으로 새로운 라운드 시작 가능."
	FinalizationNudge = "Auto recovery has reached its limit for this turn. Do not call any more tools. Summarize what was completed, what failed, and what the user should do next. The user can continue in the next message."
)
