import React, { useRef, useEffect, useState } from 'react';
import uPlot from 'uplot';
import 'uplot/dist/uPlot.min.css';
import './TimeSeriesChart.css';

function getThemeColors() {
  const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
  return {
    axisStroke: isDark ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.25)',
    gridStroke: isDark ? 'rgba(255,255,255,0.04)' : 'rgba(0,0,0,0.06)',
    tickStroke: isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.08)',
    fontColor:  isDark ? 'rgba(255,255,255,0.4)' : 'rgba(0,0,0,0.5)',
  };
}

export default function TimeSeriesChart({ title, data, series = [], height = 200, unit = '' }) {
  const wrapRef = useRef(null);
  const chartRef = useRef(null);
  const [w, setW] = useState(600);

  // Track theme changes to recreate chart with correct colors
  const [theme, setTheme] = useState(() => document.documentElement.getAttribute('data-theme') || 'light');

  useEffect(() => {
    const observer = new MutationObserver(() => {
      const t = document.documentElement.getAttribute('data-theme') || 'light';
      setTheme(t);
    });
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    return () => observer.disconnect();
  }, []);

  // Track container width
  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    setW(el.clientWidth - 2);
    const ro = new ResizeObserver((entries) => {
      const cw = entries[0]?.contentRect.width;
      if (cw > 50) setW(Math.floor(cw) - 2);
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const hasData = data && data[0] && data[0].length >= 2;

  // Create chart — recreate on theme change, size change, or series change
  useEffect(() => {
    if (!hasData || !wrapRef.current) return;

    if (chartRef.current) {
      chartRef.current.destroy();
      chartRef.current = null;
    }

    const container = wrapRef.current;
    while (container.firstChild) container.removeChild(container.firstChild);

    const colors = getThemeColors();

    const uSeries = [
      { label: 'Time' },
      ...(series.length > 0
        ? series.map((s) => ({
            label: s.label || 'Value',
            stroke: s.color || '#4a90e2',
            width: s.width || 1.5,
          }))
        : [{ label: 'Value', stroke: '#4a90e2', width: 1.5 }]),
    ];

    const axisConfig = {
      stroke: colors.axisStroke,
      grid: { stroke: colors.gridStroke, width: 1 },
      ticks: { stroke: colors.tickStroke, width: 1, size: 4 },
    };

    const opts = {
      width: w,
      height,
      cursor: { show: true, drag: { x: false, y: false } },
      select: { show: false },
      legend: { show: series.length > 1 },
      scales: {
        x: { time: true },
        y: { auto: true, range: (u, min, max) => {
          if (min === max) return [min - 1, max + 1];
          const pad = (max - min) * 0.1 || 1;
          return [min - pad, max + pad];
        }},
      },
      axes: [
        { ...axisConfig, font: `10px Inter, system-ui, sans-serif`, gap: 6 },
        { ...axisConfig, font: `10px JetBrains Mono, monospace`, gap: 8, size: 50 },
      ],
      series: uSeries,
    };

    chartRef.current = new uPlot(opts, data, container);

    return () => {
      if (chartRef.current) {
        chartRef.current.destroy();
        chartRef.current = null;
      }
    };
  }, [hasData, w, height, series.length, theme]);

  // Update data on existing chart
  useEffect(() => {
    if (chartRef.current && hasData) {
      chartRef.current.setData(data);
    }
  }, [data, hasData]);

  // Resize chart when container width changes
  useEffect(() => {
    if (chartRef.current) {
      chartRef.current.setSize({ width: w, height });
    }
  }, [w, height]);

  return (
    <div className="ts-chart">
      <div className="ts-chart__header">
        <span className="ts-chart__title">{title}</span>
        {unit && <span className="ts-chart__unit">{unit}</span>}
      </div>
      <div className="ts-chart__body" ref={wrapRef}>
        {!hasData && (
          <div className="ts-chart__waiting">
            Collecting data...
          </div>
        )}
      </div>
    </div>
  );
}
