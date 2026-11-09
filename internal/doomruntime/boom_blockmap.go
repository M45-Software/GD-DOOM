package doomruntime

import "gddoom/internal/mapdata"

// buildBoomBlockmap follows PrBoom/DSDA P_CreateBlockMap (p_setup.c): cells
// include both endpoint/intersection rules and reverse linedef insertion order.
// The in-memory cells do not depend on the WAD format's 16-bit list offsets.
func buildBoomBlockmap(m *mapdata.Map) *mapdata.BlockMap {
	if m == nil || len(m.Vertexes) == 0 {
		return nil
	}
	minx, maxx := int(m.Vertexes[0].X), int(m.Vertexes[0].X)
	miny, maxy := int(m.Vertexes[0].Y), int(m.Vertexes[0].Y)
	for _, v := range m.Vertexes {
		minx = min(minx, int(v.X))
		maxx = max(maxx, int(v.X))
		miny = min(miny, int(v.Y))
		maxy = max(maxy, int(v.Y))
	}
	width, height := (maxx-minx+128)>>7, (maxy-miny+128)>>7
	cells := make([][]int16, width*height)
	seen := make([]int, len(cells))
	for line, ld := range m.Linedefs {
		if int(ld.V1) >= len(m.Vertexes) || int(ld.V2) >= len(m.Vertexes) {
			continue
		}
		a, b := m.Vertexes[ld.V1], m.Vertexes[ld.V2]
		x1, y1, x2, y2 := int(a.X), int(a.Y), int(b.X), int(b.Y)
		dx, dy := x2-x1, y2-y1
		loX, hiX, loY, hiY := min(x1, x2), max(x1, x2), min(y1, y2), max(y1, y2)
		positive, negative := (dx^dy) > 0, (dx^dy) < 0
		add := func(x, y int) {
			if x >= 0 && x < width && y >= 0 && y < height {
				index := y*width + x
				if seen[index] != line+1 {
					seen[index] = line + 1
					cells[index] = append(cells[index], int16(uint16(line)))
				}
			}
		}
		add((x1-minx)>>7, (y1-miny)>>7)
		add((x2-minx)>>7, (y2-miny)>>7)
		if dx != 0 {
			for column := 0; column < width; column++ {
				x := minx + (column << 7)
				y := int(int32(dy)*int32(x-x1)/int32(dx)) + y1
				row, part := (y-miny)>>7, (y-miny)&127
				if row < 0 || row >= height || x < loX || x > hiX {
					continue
				}
				add(column, row)
				if part == 0 {
					switch {
					case negative:
						if row > 0 && loY < y {
							add(column, row-1)
						}
						if column > 0 && loX < x {
							add(column-1, row)
						}
					case positive:
						if row > 0 && column > 0 && loX < x {
							add(column-1, row-1)
						}
					case dy == 0:
						if column > 0 && loX < x {
							add(column-1, row)
						}
					}
				} else if column > 0 && loX < x {
					add(column-1, row)
				}
			}
		}
		if dy != 0 {
			for row := 0; row < height; row++ {
				y := miny + (row << 7)
				x := int(int32(dx)*int32(y-y1)/int32(dy)) + x1
				column, part := (x-minx)>>7, (x-minx)&127
				if column < 0 || column >= width || y < loY || y > hiY {
					continue
				}
				add(column, row)
				if part == 0 {
					switch {
					case negative:
						if row > 0 && loY < y {
							add(column, row-1)
						}
						if column > 0 && loX < x {
							add(column-1, row)
						}
					case dx == 0:
						if row > 0 && loY < y {
							add(column, row-1)
						}
					case positive:
						if column > 0 && row > 0 && loY < y {
							add(column-1, row-1)
						}
					}
				} else if row > 0 && loY < y {
					add(column, row-1)
				}
			}
		}
	}
	for i, cell := range cells {
		list := make([]int16, len(cell)+1)
		for j, line := range cell {
			list[len(cell)-j] = line
		}
		cells[i] = list
	}
	return &mapdata.BlockMap{OriginX: int16(minx), OriginY: int16(miny), Width: int16(width), Height: int16(height), Cells: cells}
}
