import React, { useState, useRef, useEffect } from 'react';
import type { StockQuote } from '../types/dashboard';
import { formatStockPrice } from '../lib/utils';
import { useStocks } from '../hooks/useStocks';
import { useSymbolSearch } from '../hooks/useSymbolSearch';

interface StocksCardProps {
  stocks: StockQuote[] | null;
  delay?: number;
}

export function StocksCard({ stocks: initialStocks, delay = 0 }: StocksCardProps): React.ReactElement {
  const { stocks, isFetching, add, remove } = useStocks(initialStocks);
  const { searchQuery, setSearchQuery, results, isSearching } = useSymbolSearch();

  const [hoveredSymbol, setHoveredSymbol] = useState<string | null>(null);
  const [isPaused, setIsPaused] = useState(false);
  const [shouldScroll, setShouldScroll] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const innerRef = useRef<HTMLDivElement>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const addContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setLoaded(true), 100 + delay * 1000);
    return () => clearTimeout(t);
  }, [delay]);

  useEffect(() => {
    if (showAddForm) searchRef.current?.focus();
  }, [showAddForm]);

  useEffect(() => {
    if (!showAddForm) return;
    function handleClick(e: MouseEvent) {
      if (
        dropdownRef.current && !dropdownRef.current.contains(e.target as Node) &&
        addContainerRef.current && !addContainerRef.current.contains(e.target as Node)
      ) {
        setShowAddForm(false);
        setSearchQuery('');
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [showAddForm, setSearchQuery]);

  useEffect(() => {
    if (stocks.length === 0) {
      setShouldScroll(false);
      return;
    }
    const container = containerRef.current;
    const inner = innerRef.current;
    if (!container || !inner) return;
    const check = () => {
      setShouldScroll(prev => {
        const contentW = prev ? inner.scrollWidth / 2 : inner.scrollWidth;
        return contentW > container.clientWidth;
      });
    };
    check();
    const observer = new ResizeObserver(check);
    observer.observe(container);
    return () => observer.disconnect();
  }, [stocks]);

  const currentSymbols = new Set(stocks.map((s) => s.symbol));
  const duration = Math.max(8, stocks.length * 4);
  const marqueeItems = [...stocks, ...stocks];

  function yahooFinanceUrl(symbol: string): string {
    const slug = symbol === 'BTC' ? 'BTC-USD' : symbol;
    return `https://finance.yahoo.com/quote/${slug}`;
  }

  function handleAdd(symbol: string) {
    add.mutate(symbol, {
      onSuccess: () => {
        setShowAddForm(false);
        setSearchQuery('');
      },
    });
  }

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      background: 'var(--bg-card)',
      border: '1px solid var(--bg-card-border)',
      borderRadius: 12,
      backdropFilter: 'blur(20px)',
      marginBottom: 20,
      overflow: 'visible',
      position: 'relative',
      zIndex: showAddForm ? 10 : undefined,
      opacity: loaded ? 1 : 0,
      transform: loaded ? 'translateY(0)' : 'translateY(12px)',
      transition: `opacity 0.7s cubic-bezier(0.16, 1, 0.3, 1) ${delay}s, transform 0.7s cubic-bezier(0.16, 1, 0.3, 1) ${delay}s`,
    }}>

      <style>{`
        @keyframes argus-ticker-scroll {
          0%   { transform: translateX(0); }
          100% { transform: translateX(-50%); }
        }
      `}</style>

      {/* Label */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 7,
        padding: '11px 16px',
        borderRight: '1px solid var(--border-subtle)',
        flexShrink: 0,
      }}>
        <span style={{ fontSize: 13, color: 'var(--text-accent)', opacity: 0.8 }}>◧</span>
        <span style={{
          fontSize: 11,
          fontWeight: 600,
          color: 'var(--text-secondary)',
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
        }}>
          Markets
        </span>
        {isFetching && (
          <span style={{ fontSize: 9, color: 'var(--text-accent)', fontFamily: "'JetBrains Mono', monospace" }}>●</span>
        )}
      </div>

      {/* Ticker strip — animated marquee */}
      <div
        ref={containerRef}
        style={{ flex: 1, overflow: 'hidden', display: 'flex', alignItems: 'center' }}
        onMouseEnter={() => setIsPaused(true)}
        onMouseLeave={() => setIsPaused(false)}
      >
        {stocks.length === 0 ? (
          <span style={{ fontSize: 12, color: 'var(--text-muted)', padding: '0 16px', whiteSpace: 'nowrap' }}>
            No tickers — add one →
          </span>
        ) : (
          <div
            ref={innerRef}
            style={{
              display: 'flex',
              alignItems: 'center',
              width: 'fit-content',
              flexShrink: 0,
              ...(shouldScroll && {
                animation: `argus-ticker-scroll ${duration}s linear infinite`,
                animationPlayState: isPaused ? 'paused' : 'running',
                willChange: 'transform',
              }),
            }}
          >
            {(shouldScroll ? marqueeItems : stocks).map((stock, i) => {
              const isPositive = stock.change >= 0;
              const changeColor = isPositive ? '#10b981' : '#ef4444';
              const isHovered = hoveredSymbol === stock.symbol;
              return (
                <React.Fragment key={`${stock.symbol}-${i}`}>
                  {i > 0 && (
                    <span style={{ width: 1, height: 18, background: 'var(--border-subtle)', flexShrink: 0 }} />
                  )}
                  <a
                    href={yahooFinanceUrl(stock.symbol)}
                    target="_blank"
                    rel="noopener noreferrer"
                    onMouseEnter={() => setHoveredSymbol(stock.symbol)}
                    onMouseLeave={() => setHoveredSymbol(null)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                      padding: '11px 14px',
                      textDecoration: 'none',
                      background: isHovered ? 'var(--bg-elevated)' : 'transparent',
                      transition: 'background 0.15s',
                      flexShrink: 0,
                      cursor: 'pointer',
                    }}
                  >
                    <span style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 11,
                      fontWeight: 600,
                      color: 'var(--text-secondary)',
                    }}>
                      {stock.symbol}
                    </span>
                    <span style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 12,
                      fontWeight: 600,
                      color: 'var(--text-clock)',
                    }}>
                      {formatStockPrice(stock.price, stock.symbol)}
                    </span>
                    <span style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 11,
                      color: changeColor,
                    }}>
                      {isPositive ? '+' : ''}{stock.pct.toFixed(2)}%
                    </span>
                    {/* Remove — always takes space to prevent layout shift */}
                    <button
                      onClick={(e) => { e.preventDefault(); remove.mutate(stock.symbol); }}
                      title={`Remove ${stock.symbol}`}
                      style={{
                        background: 'none',
                        border: 'none',
                        padding: '0 1px',
                        cursor: 'pointer',
                        fontSize: 13,
                        lineHeight: 1,
                        color: 'var(--text-secondary)',
                        opacity: isHovered ? 0.7 : 0,
                        transition: 'opacity 0.15s',
                        flexShrink: 0,
                      }}
                    >
                      ×
                    </button>
                  </a>
                </React.Fragment>
              );
            })}
          </div>
        )}
      </div>

      {/* Add ticker — anchored to the right */}
      <div
        ref={addContainerRef}
        style={{
          borderLeft: '1px solid var(--border-subtle)',
          flexShrink: 0,
          position: 'relative',
        }}
      >
        {!showAddForm ? (
          <button
            onClick={() => setShowAddForm(true)}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 5,
              padding: '11px 16px',
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              fontSize: 12,
              color: 'var(--text-secondary)',
              whiteSpace: 'nowrap',
            }}
          >
            <span style={{ fontSize: 15, lineHeight: 1 }}>+</span> Add ticker
          </button>
        ) : (
          <div style={{ display: 'flex', alignItems: 'center', padding: '7px 10px', gap: 6 }}>
            <input
              ref={searchRef}
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              maxLength={50}
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  setShowAddForm(false);
                  setSearchQuery('');
                }
              }}
              placeholder="Symbol or company…"
              style={{
                width: 190,
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-medium)',
                borderRadius: 6,
                padding: '5px 10px',
                fontSize: 12,
                color: 'var(--text-primary)',
                outline: 'none',
                caretColor: 'var(--text-accent)',
              }}
            />
            {isSearching && (
              <span style={{ fontSize: 11, color: 'var(--text-secondary)', userSelect: 'none' }}>…</span>
            )}
          </div>
        )}

        {/* Dropdown — always opens downward since the bar is at the top */}
        {results.length > 0 && showAddForm && (
          <div
            ref={dropdownRef}
            style={{
              position: 'absolute',
              top: 'calc(100% + 4px)',
              right: 0,
              width: 320,
              background: 'var(--bg-primary)',
              border: '1px solid var(--border-medium)',
              borderRadius: 10,
              zIndex: 50,
              maxHeight: 280,
              overflowY: 'auto',
            }}
          >
            {results.map((r) => {
              const alreadyAdded = currentSymbols.has(r.symbol);
              return (
                <div
                  key={r.symbol}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '9px 12px',
                    borderBottom: '1px solid var(--border-subtle)',
                    gap: 8,
                  }}
                >
                  <div style={{ minWidth: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 13,
                      fontWeight: 600,
                      color: 'var(--text-primary)',
                      flexShrink: 0,
                    }}>
                      {r.symbol}
                    </span>
                    <span
                      title={r.description}
                      style={{
                        fontSize: 12,
                        color: 'var(--text-secondary)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        minWidth: 0,
                      }}
                    >
                      {r.description}
                    </span>
                  </div>
                  {alreadyAdded ? (
                    <span style={{
                      fontSize: 11,
                      color: '#10b981',
                      flexShrink: 0,
                      padding: '2px 8px',
                      borderRadius: 4,
                      background: 'rgba(16,185,129,0.1)',
                      border: '1px solid rgba(16,185,129,0.2)',
                    }}>
                      Added
                    </span>
                  ) : (
                    <button
                      onClick={() => handleAdd(r.symbol)}
                      disabled={add.isPending}
                      style={{
                        flexShrink: 0,
                        background: 'rgba(99,102,241,0.12)',
                        border: '1px solid rgba(99,102,241,0.25)',
                        borderRadius: 6,
                        padding: '3px 10px',
                        fontSize: 12,
                        color: 'var(--text-accent)',
                        cursor: add.isPending ? 'not-allowed' : 'pointer',
                        opacity: add.isPending ? 0.6 : 1,
                      }}
                    >
                      + Add
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
