package doomruntime

import "fmt"

func (w saveBinaryWriter) boomSectorTouchSlice(nodes []boomSectorTouchState) error {
	if err := w.bool(nodes != nil); err != nil {
		return err
	}
	if nodes == nil {
		return nil
	}
	if err := w.u32(uint32(len(nodes))); err != nil {
		return err
	}
	for _, node := range nodes {
		if err := w.int(node.Sector); err != nil {
			return err
		}
		if err := w.int(node.Actor); err != nil {
			return err
		}
	}
	return nil
}

func (r saveBinaryReader) boomSectorTouchSlice() ([]boomSectorTouchState, error) {
	present, err := r.bool()
	if err != nil || !present {
		return nil, err
	}
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	if uint64(n) > uint64(r.remaining()/8) {
		return nil, fmt.Errorf("truncated Boom sector membership")
	}
	out := make([]boomSectorTouchState, n)
	for i := range out {
		if out[i].Sector, err = r.int(); err != nil {
			return nil, err
		}
		if out[i].Actor, err = r.int(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
