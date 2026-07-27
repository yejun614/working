package types

// Board는 칸반 보드 하나를 나타낸다.
type Board struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Column은 보드 안에서 카드의 상태와 순서를 나타낸다.
type Column struct {
	ID        string `json:"id"`
	BoardID   string `json:"boardId"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	CreatedAt string `json:"createdAt"`
}

// Card는 칸반 보드에서 관리하는 업무를 나타낸다.
type Card struct {
	ID          string          `json:"id"`
	BoardID     string          `json:"boardId"`
	ColumnID    string          `json:"columnId"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	DueDate     string          `json:"dueDate,omitempty"`
	Priority    string          `json:"priority,omitempty"`
	Labels      []string        `json:"labels,omitempty"`
	Checklist   []ChecklistItem `json:"checklist,omitempty"`
	Assignee    string          `json:"assignee,omitempty"`
	Attachments []string        `json:"attachments,omitempty"`
	Position    int             `json:"position"`
	Archived    bool            `json:"archived"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

// ChecklistItem은 카드 안의 하위 업무를 나타낸다.
type ChecklistItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

// DueEvent는 캘린더 모듈이 칸반 카드의 마감일을 표시할 때 사용하는 읽기 모델이다.
type DueEvent struct {
	CardID  string `json:"cardId"`
	Title   string `json:"title"`
	DueDate string `json:"dueDate"`
}
