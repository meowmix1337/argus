import React, { useEffect, useState } from 'react';

interface CardProps {
  children: React.ReactNode;
  delay?: number;
  span?: number;
  className?: string;
  noGridSpan?: boolean;
}

export function Card({ children, delay = 0, span = 1, className = '', noGridSpan = false }: CardProps): React.ReactElement {
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setLoaded(true), 100 + delay * 1000);
    return () => clearTimeout(timer);
  }, [delay]);

  return (
    <div
      style={{
        ...(noGridSpan ? {} : { gridColumn: `span ${span}` }),
        background: 'var(--bg-card)',
        border: '1px solid var(--bg-card-border)',
        borderRadius: 16,
        padding: 24,
        backdropFilter: 'blur(20px)',
        height: '100%',
        overflow: 'hidden',
        display: 'flex',
        flexDirection: 'column',
        opacity: loaded ? 1 : 0,
        transform: loaded ? 'translateY(0)' : 'translateY(16px)',
        // Once loaded, remove transform from transition so dnd-kit can own the transform property
        // without fighting the Card's own CSS transition during drag operations.
        transition: loaded
          ? `opacity 0.7s cubic-bezier(0.16, 1, 0.3, 1) ${delay}s`
          : `opacity 0.7s cubic-bezier(0.16, 1, 0.3, 1) ${delay}s, transform 0.7s cubic-bezier(0.16, 1, 0.3, 1) ${delay}s`,
      }}
      className={className}
    >
      {children}
    </div>
  );
}
