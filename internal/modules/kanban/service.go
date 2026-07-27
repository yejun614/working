// Package kanban은 칸반 보드와 카드의 비즈니스 로직을 제공한다.
package kanban

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"working/internal/modules/kanban/store"
	"working/internal/modules/kanban/types"
)

// Service는 Wails를 통해 프론트엔드에 노출되는 칸반 서비스다.
type Service struct {
	store *store.Store
}

// NewService는 칸반 서비스와 저장소를 초기화한다.
func NewService() (*Service, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	return &Service{store: st}, nil
}

func (s *Service) ServiceShutdown() {}

// BoardList는 보드 목록을 생성 순서로 반환한다.
func (s *Service) BoardList() ([]types.Board, error) {
	d, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return d.Boards, nil
}

// BoardGet은 보드와 그 보드에 속한 컬럼·카드를 함께 반환한다.
func (s *Service) BoardGet(id string) (*BoardView, error) {
	d, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	for _, board := range d.Boards {
		if board.ID == id {
			view := &BoardView{Board: board}
			for _, col := range d.Columns {
				if col.BoardID == id {
					view.Columns = append(view.Columns, col)
				}
			}
			for _, card := range d.Cards {
				if card.BoardID == id && !card.Archived {
					view.Cards = append(view.Cards, card)
				}
			}
			sort.Slice(view.Columns, func(i, j int) bool { return view.Columns[i].Position < view.Columns[j].Position })
			sort.Slice(view.Cards, func(i, j int) bool { return view.Cards[i].Position < view.Cards[j].Position })
			return view, nil
		}
	}
	return nil, fmt.Errorf("보드를 찾을 수 없습니다: %s", id)
}

// BoardView는 보드 화면에 필요한 데이터를 묶는다.
type BoardView struct {
	Board   types.Board    `json:"board"`
	Columns []types.Column `json:"columns"`
	Cards   []types.Card   `json:"cards"`
}

// BoardCreate는 기본 컬럼(할 일, 진행 중, 완료)을 포함한 보드를 만든다.
func (s *Service) BoardCreate(name string) (*types.Board, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("보드 이름은 필수입니다")
	}
	d, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	now := timestamp()
	board := types.Board{ID: newID(), Name: name, CreatedAt: now, UpdatedAt: now}
	d.Boards = append(d.Boards, board)
	for i, name := range []string{"할 일", "진행 중", "완료"} {
		d.Columns = append(d.Columns, types.Column{ID: newID(), BoardID: board.ID, Name: name, Position: i, CreatedAt: now})
	}
	if err := s.store.Save(d); err != nil {
		return nil, err
	}
	return &board, nil
}

// BoardUpdate는 보드 이름을 변경한다.
func (s *Service) BoardUpdate(board types.Board) error {
	if strings.TrimSpace(board.Name) == "" {
		return fmt.Errorf("보드 이름은 필수입니다")
	}
	d, err := s.store.Load()
	if err != nil {
		return err
	}
	for i := range d.Boards {
		if d.Boards[i].ID == board.ID {
			d.Boards[i].Name = strings.TrimSpace(board.Name)
			d.Boards[i].UpdatedAt = timestamp()
			return s.store.Save(d)
		}
	}
	return fmt.Errorf("보드를 찾을 수 없습니다: %s", board.ID)
}

// BoardDelete는 보드와 그 하위 컬럼·카드를 함께 삭제한다.
func (s *Service) BoardDelete(id string) error {
	d, err := s.store.Load()
	if err != nil {
		return err
	}
	found := false
	boards := d.Boards[:0]
	for _, b := range d.Boards {
		if b.ID == id {
			found = true
		} else {
			boards = append(boards, b)
		}
	}
	if !found {
		return fmt.Errorf("보드를 찾을 수 없습니다: %s", id)
	}
	d.Boards = boards
	cols := d.Columns[:0]
	for _, c := range d.Columns {
		if c.BoardID != id {
			cols = append(cols, c)
		}
	}
	d.Columns = cols
	cards := d.Cards[:0]
	for _, c := range d.Cards {
		if c.BoardID != id {
			cards = append(cards, c)
		}
	}
	d.Cards = cards
	return s.store.Save(d)
}

// ColumnCreate는 보드의 마지막에 컬럼을 추가한다.
func (s *Service) ColumnCreate(column types.Column) (*types.Column, error) {
	name := strings.TrimSpace(column.Name)
	if name == "" {
		return nil, fmt.Errorf("컬럼 이름은 필수입니다")
	}
	d, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	if !hasBoard(d, column.BoardID) {
		return nil, fmt.Errorf("보드를 찾을 수 없습니다: %s", column.BoardID)
	}
	max := -1
	for _, c := range d.Columns {
		if c.BoardID == column.BoardID && c.Position > max {
			max = c.Position
		}
	}
	column.ID, column.Name, column.Position, column.CreatedAt = newID(), name, max+1, timestamp()
	d.Columns = append(d.Columns, column)
	if err := s.store.Save(d); err != nil {
		return nil, err
	}
	return &column, nil
}

// ColumnUpdate는 컬럼 이름과 순서를 변경한다.
func (s *Service) ColumnUpdate(column types.Column) error {
	d, err := s.store.Load()
	if err != nil {
		return err
	}
	for i := range d.Columns {
		if d.Columns[i].ID == column.ID {
			if strings.TrimSpace(column.Name) == "" {
				return fmt.Errorf("컬럼 이름은 필수입니다")
			}
			d.Columns[i].Name = strings.TrimSpace(column.Name)
			d.Columns[i].Position = column.Position
			return s.store.Save(d)
		}
	}
	return fmt.Errorf("컬럼을 찾을 수 없습니다: %s", column.ID)
}

// ColumnMove는 같은 보드 안에서 컬럼을 지정 위치로 이동하고 순번을 정규화한다.
func (s *Service) ColumnMove(id string, position int) error {
	d, err := s.store.Load()
	if err != nil {
		return err
	}
	columnIndex := -1
	boardID := ""
	for i := range d.Columns {
		if d.Columns[i].ID == id {
			columnIndex = i
			boardID = d.Columns[i].BoardID
			break
		}
	}
	if columnIndex < 0 {
		return fmt.Errorf("컬럼을 찾을 수 없습니다: %s", id)
	}

	boardColumns := make([]types.Column, 0)
	otherColumns := make([]types.Column, 0, len(d.Columns)-1)
	for i, column := range d.Columns {
		if column.BoardID != boardID {
			continue
		}
		if i == columnIndex {
			continue
		}
		boardColumns = append(boardColumns, column)
	}
	for _, column := range d.Columns {
		if column.BoardID != boardID {
			otherColumns = append(otherColumns, column)
		}
	}
	moved := d.Columns[columnIndex]
	sort.SliceStable(boardColumns, func(i, j int) bool { return boardColumns[i].Position < boardColumns[j].Position })
	if position < 0 {
		position = 0
	}
	if position > len(boardColumns) {
		position = len(boardColumns)
	}
	boardColumns = append(boardColumns, types.Column{})
	copy(boardColumns[position+1:], boardColumns[position:])
	boardColumns[position] = moved
	for i := range boardColumns {
		boardColumns[i].Position = i
	}
	d.Columns = append(otherColumns, boardColumns...)
	return s.store.Save(d)
}

// ColumnDelete는 카드가 남아 있지 않은 컬럼만 삭제한다.
func (s *Service) ColumnDelete(id string) error {
	d, err := s.store.Load()
	if err != nil {
		return err
	}
	for _, c := range d.Cards {
		if c.ColumnID == id {
			return fmt.Errorf("카드를 다른 컬럼으로 먼저 이동하세요")
		}
	}
	for i, c := range d.Columns {
		if c.ID == id {
			d.Columns = append(d.Columns[:i], d.Columns[i+1:]...)
			return s.store.Save(d)
		}
	}
	return fmt.Errorf("컬럼을 찾을 수 없습니다: %s", id)
}

// CardList는 보드의 활성 카드만 반환한다.
func (s *Service) CardList(boardID string) ([]types.Card, error) {
	view, err := s.BoardGet(boardID)
	if err != nil {
		return nil, err
	}
	return view.Cards, nil
}

// ArchivedCardList는 보드의 아카이브 카드만 반환한다.
func (s *Service) ArchivedCardList(boardID string) ([]types.Card, error) {
	d, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	var cards []types.Card
	for _, c := range d.Cards {
		if c.BoardID == boardID && c.Archived {
			cards = append(cards, c)
		}
	}
	return cards, nil
}

// CardSave는 신규 카드와 기존 카드의 공통 저장 메서드다.
func (s *Service) CardSave(card types.Card) (*types.Card, error) {
	if strings.TrimSpace(card.Title) == "" {
		return nil, fmt.Errorf("카드 제목은 필수입니다")
	}
	d, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	if !hasBoard(d, card.BoardID) || !hasColumn(d, card.ColumnID, card.BoardID) {
		return nil, fmt.Errorf("보드 또는 컬럼을 찾을 수 없습니다")
	}
	now := timestamp()
	if card.ID == "" {
		card.ID, card.CreatedAt = newID(), now
		card.UpdatedAt = now
		card.Position = nextCardPosition(d, card.ColumnID)
		d.Cards = append(d.Cards, card)
	} else {
		found := false
		for i := range d.Cards {
			if d.Cards[i].ID == card.ID {
				card.CreatedAt = d.Cards[i].CreatedAt
				card.UpdatedAt = now
				d.Cards[i] = card
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("카드를 찾을 수 없습니다: %s", card.ID)
		}
	}
	if err := s.store.Save(d); err != nil {
		return nil, err
	}
	return &card, nil
}

// CardDelete는 카드를 영구 삭제한다. 일반 화면에서는 아카이브를 사용한다.
func (s *Service) CardDelete(id string) error { return s.setArchived(id, false, true) }

// CardArchive는 카드를 일반 보드에서 숨긴다.
func (s *Service) CardArchive(id string) error { return s.setArchived(id, true, false) }

// CardRestore는 아카이브 카드를 보드에 다시 표시한다.
func (s *Service) CardRestore(id string) error { return s.setArchived(id, false, false) }

func (s *Service) setArchived(id string, archived, permanent bool) error {
	d, err := s.store.Load()
	if err != nil {
		return err
	}
	for i := range d.Cards {
		if d.Cards[i].ID == id {
			if permanent {
				d.Cards = append(d.Cards[:i], d.Cards[i+1:]...)
			}
			if !permanent {
				d.Cards[i].Archived = archived
				d.Cards[i].UpdatedAt = timestamp()
			}
			return s.store.Save(d)
		}
	}
	return fmt.Errorf("카드를 찾을 수 없습니다: %s", id)
}

// CardMove는 카드를 지정 컬럼의 지정 위치로 이동하고,
// 이동 전·후 컬럼의 나머지 카드 순번도 연속된 값으로 다시 정렬한다.
func (s *Service) CardMove(id, columnID string, position int) error {
	d, err := s.store.Load()
	if err != nil {
		return err
	}
	for i := range d.Cards {
		if d.Cards[i].ID != id {
			continue
		}
		if !hasColumn(d, columnID, d.Cards[i].BoardID) {
			return fmt.Errorf("컬럼을 찾을 수 없습니다: %s", columnID)
		}
		fromColumnID := d.Cards[i].ColumnID
		card := d.Cards[i]
		card.ColumnID = columnID
		card.UpdatedAt = timestamp()
		d.Cards = append(d.Cards[:i], d.Cards[i+1:]...)
		reorderCards(d, fromColumnID, columnID, &card, position)
		d.Cards = append(d.Cards, card)
		return s.store.Save(d)
	}
	return fmt.Errorf("카드를 찾을 수 없습니다: %s", id)
}

// reorderCards는 이동 카드를 제외한 활성 카드 목록에 이동 카드를 삽입하고
// 출발·도착 컬럼의 Position을 0부터 다시 부여한다.
func reorderCards(d *store.Data, fromColumnID, toColumnID string, moved *types.Card, position int) {
	from := make([]types.Card, 0)
	to := make([]types.Card, 0)
	for _, card := range d.Cards {
		if card.Archived {
			continue
		}
		if card.ColumnID == fromColumnID {
			from = append(from, card)
		}
		if card.ColumnID == toColumnID {
			to = append(to, card)
		}
	}
	sort.SliceStable(from, func(i, j int) bool { return from[i].Position < from[j].Position })
	sort.SliceStable(to, func(i, j int) bool { return to[i].Position < to[j].Position })
	if fromColumnID == toColumnID {
		to = from
	}
	if position < 0 {
		position = 0
	}
	if position > len(to) {
		position = len(to)
	}
	to = append(to, types.Card{})
	copy(to[position+1:], to[position:])
	to[position] = *moved

	positions := make(map[string]int, len(from)+len(to))
	for i, card := range from {
		positions[card.ID] = i
	}
	for i, card := range to {
		positions[card.ID] = i
	}
	for i := range d.Cards {
		if position, ok := positions[d.Cards[i].ID]; ok {
			d.Cards[i].Position = position
		}
	}
	moved.Position = position
}

// DueEvents는 마감일이 있는 활성 카드만 캘린더 표시용으로 반환한다.
func (s *Service) DueEvents() ([]types.DueEvent, error) {
	d, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	var out []types.DueEvent
	for _, c := range d.Cards {
		if !c.Archived && c.DueDate != "" {
			out = append(out, types.DueEvent{CardID: c.ID, Title: c.Title, DueDate: c.DueDate})
		}
	}
	return out, nil
}

func hasBoard(d *store.Data, id string) bool {
	for _, board := range d.Boards {
		if board.ID == id {
			return true
		}
	}
	return false
}

func hasColumn(d *store.Data, id, boardID string) bool {
	for _, column := range d.Columns {
		if column.ID == id && column.BoardID == boardID {
			return true
		}
	}
	return false
}

func nextCardPosition(d *store.Data, columnID string) int {
	max := -1
	for _, card := range d.Cards {
		if card.ColumnID == columnID && card.Position > max {
			max = card.Position
		}
	}
	return max + 1
}

func timestamp() string { return time.Now().UTC().Format(time.RFC3339) }

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
