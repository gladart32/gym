package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const (
	sourceName    = "exercises-dataset"
	mediaSource   = "gymvisual"
	mediaSize     = 180
	defaultAttrib = "© Gym visual — https://gymvisual.com/"
)

type stats struct {
	Inserted, Updated, Skipped int
	ByTracking                 map[string]int
}

type importer struct {
	tx      *sql.Tx
	src     *sourceData
	update  bool
	prefix  string
	bodyIDs map[string]int64
	equipID map[string]int64
	muscIDs map[string]int64
}

// upsertDict вставляет/обновляет справочник и возвращает code -> id.
func (im *importer) upsertDict(ctx context.Context, table string, items map[string]string) (map[string]int64, error) {
	q := fmt.Sprintf(`INSERT INTO %s (code, name_en, name_ru) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE name_en = VALUES(name_en), name_ru = VALUES(name_ru)`, table)
	stmt, err := im.tx.PrepareContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	codes := make([]string, 0, len(items))
	for c := range items {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		if _, err := stmt.ExecContext(ctx, c, titleFirst(c), items[c]); err != nil {
			return nil, fmt.Errorf("%s %q: %w", table, c, err)
		}
	}

	rows, err := im.tx.QueryContext(ctx, fmt.Sprintf(`SELECT code, id FROM %s`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make(map[string]int64)
	for rows.Next() {
		var c string
		var id int64
		if err := rows.Scan(&c, &id); err != nil {
			return nil, err
		}
		ids[c] = id
	}
	return ids, rows.Err()
}

func (im *importer) importDictionaries(ctx context.Context) error {
	var err error
	if im.bodyIDs, err = im.upsertDict(ctx, "body_parts", im.src.BodyParts); err != nil {
		return err
	}
	if im.equipID, err = im.upsertDict(ctx, "equipment", im.src.Equipment); err != nil {
		return err
	}
	// мышцы: только канонические коды, синонимы схлопываются
	canon := make(map[string]string)
	for _, m := range im.src.Muscles {
		canon[m.Canonical] = im.src.Muscles[m.Canonical].RU
	}
	im.muscIDs, err = im.upsertDict(ctx, "muscles", canon)
	return err
}

type preparedStmts struct {
	findEx, insEx, updEx, upsTr, delSec, insSec, delMedia, insMedia *sql.Stmt
}

func (im *importer) prepare(ctx context.Context) (*preparedStmts, error) {
	p := &preparedStmts{}
	defs := []struct {
		dst **sql.Stmt
		q   string
	}{
		{&p.findEx, `SELECT id FROM exercises WHERE source = ? AND external_id = ?`},
		{&p.insEx, `INSERT INTO exercises (source, external_id, body_part_id, equipment_id, target_muscle_id, tracking_type)
			VALUES (?, ?, ?, ?, ?, ?)`},
		{&p.updEx, `UPDATE exercises SET body_part_id = ?, equipment_id = ?, target_muscle_id = ?, tracking_type = ?
			WHERE id = ?`},
		{&p.upsTr, `INSERT INTO exercise_translations (exercise_id, lang, name, instructions, steps) VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE name = VALUES(name), instructions = VALUES(instructions), steps = VALUES(steps)`},
		{&p.delSec, `DELETE FROM exercise_secondary_muscles WHERE exercise_id = ?`},
		{&p.insSec, `INSERT INTO exercise_secondary_muscles (exercise_id, muscle_id) VALUES (?, ?)`},
		{&p.delMedia, `DELETE FROM exercise_media WHERE exercise_id = ? AND source = ?`},
		{&p.insMedia, `INSERT INTO exercise_media (exercise_id, type, path, width, height, source, attribution, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0)`},
	}
	for _, d := range defs {
		s, err := im.tx.PrepareContext(ctx, d.q)
		if err != nil {
			p.close()
			return nil, err
		}
		*d.dst = s
	}
	return p, nil
}

func (p *preparedStmts) close() {
	for _, s := range []*sql.Stmt{p.findEx, p.insEx, p.updEx, p.upsTr, p.delSec, p.insSec, p.delMedia, p.insMedia} {
		if s != nil {
			s.Close()
		}
	}
}

func (im *importer) importExercises(ctx context.Context) (*stats, error) {
	p, err := im.prepare(ctx)
	if err != nil {
		return nil, err
	}
	defer p.close()

	st := &stats{ByTracking: map[string]int{}}
	for _, e := range im.src.Exercises {
		target, secondary := im.src.canonicalMuscles(e)
		bodyID := im.bodyIDs[e.Category]
		equipID := im.equipID[e.Equipment]
		targetID := im.muscIDs[target]
		track := trackingType(e)

		var id int64
		err := p.findEx.QueryRowContext(ctx, sourceName, e.ID).Scan(&id)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			res, err := p.insEx.ExecContext(ctx, sourceName, e.ID, bodyID, equipID, targetID, track)
			if err != nil {
				return nil, fmt.Errorf("exercise %s: %w", e.ID, err)
			}
			if id, err = res.LastInsertId(); err != nil {
				return nil, err
			}
			st.Inserted++
		case err != nil:
			return nil, err
		case !im.update:
			st.Skipped++ // уже импортировано — не трогаем, чтобы не затереть правки администратора
			continue
		default:
			if _, err := p.updEx.ExecContext(ctx, bodyID, equipID, targetID, track, id); err != nil {
				return nil, fmt.Errorf("exercise %s: %w", e.ID, err)
			}
			st.Updated++
		}
		st.ByTracking[track]++

		if err := im.writeDetails(ctx, p, id, e, secondary); err != nil {
			return nil, fmt.Errorf("exercise %s: %w", e.ID, err)
		}
	}
	return st, nil
}

func (im *importer) writeDetails(ctx context.Context, p *preparedStmts, id int64, e rawExercise, secondary []string) error {
	// переводы
	ru := im.src.RU[e.ID]
	ruInstr := ru.Instructions
	if ruInstr == "" {
		ruInstr = e.Instructions["ru"]
	}
	trs := []struct{ lang, name, instr string }{
		{"en", fixName(e.Name), e.Instructions["en"]},
		{"ru", ru.Name, ruInstr},
	}
	for _, t := range trs {
		steps, err := json.Marshal(e.InstructionSteps[t.lang])
		if err != nil {
			return err
		}
		if _, err := p.upsTr.ExecContext(ctx, id, t.lang, t.name, nullStr(t.instr), steps); err != nil {
			return fmt.Errorf("translation %s: %w", t.lang, err)
		}
	}

	// вспомогательные мышцы
	if _, err := p.delSec.ExecContext(ctx, id); err != nil {
		return err
	}
	for _, m := range secondary {
		if _, err := p.insSec.ExecContext(ctx, id, im.muscIDs[m]); err != nil {
			return fmt.Errorf("secondary %s: %w", m, err)
		}
	}

	// медиа (только из этого источника, свои загрузки администратора не трогаем)
	if _, err := p.delMedia.ExecContext(ctx, id, mediaSource); err != nil {
		return err
	}
	attrib := e.Attribution
	if attrib == "" {
		attrib = defaultAttrib
	}
	for _, m := range []struct{ typ, path string }{{"gif", e.GifURL}, {"thumbnail", e.Image}} {
		if _, err := p.insMedia.ExecContext(ctx, id, m.typ, im.prefix+m.path, mediaSize, mediaSize, mediaSource, attrib); err != nil {
			return fmt.Errorf("media %s: %w", m.typ, err)
		}
	}
	return nil
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
