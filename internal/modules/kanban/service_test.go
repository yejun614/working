package kanban

import (
	"testing"

	"working/internal/modules/kanban/store"
	"working/internal/modules/kanban/types"
)

func TestReorderCardsWithinColumn(t *testing.T) {
	d := &store.Data{Cards: []types.Card{
		{ID: "a", ColumnID: "todo", Position: 0},
		{ID: "b", ColumnID: "todo", Position: 1},
	}}
	moved := d.Cards[1]
	d.Cards = d.Cards[:1]
	reorderCards(d, "todo", "todo", &moved, 0)
	d.Cards = append(d.Cards, moved)

	if d.Cards[0].Position != 1 || d.Cards[1].Position != 0 {
		t.Fatalf("same-column order was not updated: %+v", d.Cards)
	}
}

func TestReorderCardsAcrossColumns(t *testing.T) {
	d := &store.Data{Cards: []types.Card{
		{ID: "a", ColumnID: "todo", Position: 0},
		{ID: "b", ColumnID: "doing", Position: 0},
		{ID: "c", ColumnID: "doing", Position: 1},
	}}
	moved := d.Cards[0]
	d.Cards = d.Cards[1:]
	moved.ColumnID = "doing"
	reorderCards(d, "todo", "doing", &moved, 1)
	d.Cards = append(d.Cards, moved)

	for _, card := range d.Cards {
		if card.ID == "b" && card.Position != 0 {
			t.Fatalf("existing target card moved incorrectly: %+v", card)
		}
		if card.ID == "c" && card.Position != 2 {
			t.Fatalf("target card was not shifted: %+v", card)
		}
		if card.ID == "a" && card.Position != 1 {
			t.Fatalf("moved card position was not retained: %+v", card)
		}
	}
}
