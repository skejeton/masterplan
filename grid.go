package main

import (
	"math"

	"github.com/veandco/go-sdl2/sdl"
)

type GridCell struct {
	Cards []*Card
}

func NewGridCell() *GridCell {
	return &GridCell{
		Cards: []*Card{},
	}
}

func (cell *GridCell) Contains(card *Card) bool {

	for _, c := range cell.Cards {
		if card == c {
			return true
		}
	}
	return false

}

func (cell *GridCell) Add(card *Card) {
	for _, c := range cell.Cards {
		if c == card {
			return
		}
	}
	cell.Cards = append(cell.Cards, card)
}

func (cell *GridCell) Remove(card *Card) {
	for i, c := range cell.Cards {
		if card == c {
			cell.Cards[i] = nil
			cell.Cards = append(cell.Cards[:i], cell.Cards[i+1:]...)
			return
		}
	}
}

type GridSelection struct {
	Start, End Point
	Grid       *Grid
}

func NewGridSelection(x, y, x2, y2 float32, grid *Grid) GridSelection {
	selection := GridSelection{
		Start: Point{x, y},
		End:   Point{x2, y2},
		Grid:  grid,
	}
	return selection
}

func (selection GridSelection) Add(card *Card) {
	for _, cell := range selection.Cells() {
		cell.Add(card)
	}
}

func (selection GridSelection) Remove(card *Card) {
	for _, cell := range selection.Cells() {
		cell.Remove(card)
	}
}

func (selection GridSelection) Cards() []*Card {

	cards := []*Card{}
	addedMap := map[*Card]bool{}

	for _, cell := range selection.Cells() {

		for _, card := range cell.Cards {

			if _, added := addedMap[card]; !added {
				cards = append(cards, cell.Cards...)
				addedMap[card] = true
				continue
			}

		}

	}

	return cards

}

func (selection GridSelection) Cells() []*GridCell {

	cells := []*GridCell{}

	offsetY := len(selection.Grid.Cells) / 2
	offsetX := len(selection.Grid.Cells[0]) / 2

	for y := selection.Start.Y; y < selection.End.Y; y++ {
		for x := selection.Start.X; x < selection.End.X; x++ {

			cells = append(cells, selection.Grid.Cells[int(y)+offsetY][int(x)+offsetX])

		}
	}
	return cells

}

func (selection GridSelection) Valid() bool {
	return selection.Grid != nil
}

type Grid struct {
	// Cells map[Point][]*Card
	Cells [][]*GridCell
}

func NewGrid() *Grid {
	grid := &Grid{
		// Spaces: map[Point][]*Card{},
		Cells: [][]*GridCell{},
	}
	grid.Resize(1000, 1000) // For now, this will do
	return grid
}

func (grid *Grid) Resize(w, h int) {

	// By using make(), we avoid having to reallocate the array after we add enough elements that it has to be resized.
	spaces := make([][]*GridCell, 0, h)

	for y := 0; y < h; y++ {
		spaces = append(spaces, []*GridCell{})
		spaces[y] = make([]*GridCell, 0, w)
		for x := 0; x < w; x++ {
			spaces[y] = append(spaces[y], NewGridCell())
		}
	}

	grid.Cells = spaces

}

func (grid *Grid) Put(card *Card) {

	grid.Remove(card)

	card.GridExtents = grid.Select(card.Rect)

	card.GridExtents.Add(card)

}

func (grid *Grid) Remove(card *Card) {

	// Remove the extents from the previous position if it were specified
	if card.GridExtents.Valid() {
		card.GridExtents.Remove(card)
	}

}

// CardRectToGrid converts the card's rectangle to two absolute grid points representing the card's extents in absolute grid spaces.
// func (grid *Grid) CardRectToGrid(rect *sdl.FRect) []Point {

// 	return []Point{
// 		{grid.LockPosition(rect.X), grid.LockPosition(rect.Y)},
// 		{grid.LockPosition(rect.X + rect.W), grid.LockPosition(rect.Y + rect.H)},
// 	}

// }

func (grid *Grid) Select(rect *sdl.FRect) GridSelection {

	return NewGridSelection(
		grid.LockPosition(rect.X),
		grid.LockPosition(rect.Y),
		grid.LockPosition(rect.X+rect.W),
		grid.LockPosition(rect.Y+rect.H),
		grid,
	)

}

func (grid *Grid) LockPosition(position float32) float32 {

	return float32(math.Floor(float64(position / globals.GridSize)))

}

// func (grid *Grid) CardsAt(point Point) []*Card {
// 	cards := []*Card{}
// 	added := map[*Card]bool{}

// 	for _, point := range points {

// 		if existing, ok := grid.Cells[point]; ok {

// 			for _, card := range existing {

// 				if _, addedCard := added[card]; !addedCard {
// 					added[card] = true
// 					cards = append(cards, card)
// 				}

// 			}

// 		}

// 	}
// 	return cards
// }

func (grid *Grid) CardsAbove(card *Card) []*Card {

	cards := []*Card{}

	selection := grid.Select(&sdl.FRect{card.Rect.X, card.Rect.Y - globals.GridSize, card.Rect.W, globals.GridSize})

	for _, c := range selection.Cards() {
		if card != c {
			cards = append(cards, c)
		}
	}

	return cards

}

func (grid *Grid) CardsBelow(card *Card) []*Card {

	cards := []*Card{}

	selection := grid.Select(&sdl.FRect{card.Rect.X, card.Rect.Y + card.Rect.H, card.Rect.W, globals.GridSize})

	for _, c := range selection.Cards() {
		if card != c {
			cards = append(cards, c)
		}
	}

	return cards

}
