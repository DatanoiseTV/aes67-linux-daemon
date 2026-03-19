import React, { useCallback, useRef } from 'react';
import CrosspointNode from './CrosspointNode';

// Normalize a channel entry: API returns strings like "1" or objects like {id: 0, label: "L"}
function normCh(ch, index) {
  if (typeof ch === 'string' || typeof ch === 'number') {
    return { id: index, label: String(ch) };
  }
  return { id: ch.id ?? index, label: ch.label ?? ch.id ?? String(index) };
}

export default function MatrixGrid({ inputs, outputs, routes, onToggle }) {
  const tableRef = useRef(null);
  const highlightedCol = useRef(-1);

  // Flatten outputs into columns: [{groupId, groupName, ch: {id, label}}]
  const outCols = [];
  outputs.forEach((out) => {
    (out.channels || []).forEach((ch, ci) => {
      outCols.push({
        groupId: out.id,
        groupName: out.name,
        ch: normCh(ch, ci),
      });
    });
  });

  // Flatten inputs into rows grouped by source
  const inGroups = inputs.map((inp) => ({
    id: inp.id,
    name: inp.name,
    channels: (inp.channels || []).map((ch, ci) => normCh(ch, ci)),
  }));

  const totalOutCols = outCols.length;

  const handleMouseOver = useCallback((e) => {
    const td = e.target.closest('td, th');
    if (!td) return;
    const colIdx = td.cellIndex;
    if (colIdx === highlightedCol.current) return;

    const table = tableRef.current;
    if (!table) return;

    if (highlightedCol.current >= 0) {
      const old = table.querySelectorAll('.col-hl');
      old.forEach((el) => el.classList.remove('col-hl'));
    }

    if (colIdx > 0) {
      const rows = table.rows;
      for (let i = 0; i < rows.length; i++) {
        const cell = rows[i].cells[colIdx];
        if (cell) cell.classList.add('col-hl');
      }
    }
    highlightedCol.current = colIdx;
  }, []);

  const handleMouseLeave = useCallback(() => {
    const table = tableRef.current;
    if (!table) return;
    const old = table.querySelectorAll('.col-hl');
    old.forEach((el) => el.classList.remove('col-hl'));
    highlightedCol.current = -1;
  }, []);

  // Build output group headers with colspan
  const outGroupHeaders = [];
  let i = 0;
  while (i < outCols.length) {
    const gid = outCols[i].groupId;
    let span = 0;
    while (i + span < outCols.length && outCols[i + span].groupId === gid) span++;
    outGroupHeaders.push({ id: gid, name: outCols[i].groupName, span });
    i += span;
  }

  return (
    <table
      className="matrix-table"
      ref={tableRef}
      onMouseOver={handleMouseOver}
      onMouseLeave={handleMouseLeave}
    >
      <thead>
        <tr>
          <th className="matrix-corner" rowSpan={2}>
            <div className="corner-labels">
              <span className="corner-out">OUTPUTS &rarr;</span>
              <span className="corner-in">&darr; INPUTS</span>
            </div>
          </th>
          {outGroupHeaders.map((g) => (
            <th key={g.id} className="out-group" colSpan={g.span}>
              {g.name}
            </th>
          ))}
        </tr>
        <tr>
          {outCols.map((col, ci) => (
            <th key={`${col.groupId}-${ci}`} className="out-ch">
              {col.ch.label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {inGroups.map((group, gi) => {
          const altClass = gi % 2 === 1 ? ' grp-alt' : '';
          return (
            <React.Fragment key={group.id}>
              <tr className={`in-group-row${altClass}`}>
                <td className="in-group-label" colSpan={totalOutCols + 1}>
                  {group.name}
                </td>
              </tr>
              {group.channels.map((ch) => (
                <tr key={`${group.id}-${ch.id}`} className={`in-ch-row${altClass}`}>
                  <td className="in-label">{ch.label}</td>
                  {outCols.map((col) => {
                    const key = `${group.id}:${ch.id}-${col.groupId}:${col.ch.id}`;
                    const active = routes.has(key);
                    return (
                      <td key={key} className="xp-cell">
                        <CrosspointNode
                          active={active}
                          onToggle={() =>
                            onToggle(
                              { id: group.id, ch: ch.id },
                              { id: col.groupId, ch: col.ch.id }
                            )
                          }
                        />
                      </td>
                    );
                  })}
                </tr>
              ))}
            </React.Fragment>
          );
        })}
      </tbody>
    </table>
  );
}
