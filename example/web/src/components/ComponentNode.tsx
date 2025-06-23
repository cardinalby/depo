import React, { memo } from 'react';
import { Handle, Position } from 'react-flow-renderer';
import { Component, ComponentStatus } from '../types/api';

interface ComponentNodeProps {
  data: {
    component: Component;
    onClick: (component: Component) => void;
  };
}

const getStatusColor = (component: Component): string => {
    const status: ComponentStatus = component.status;
  switch (status) {
    case 'non_runnable': return '#ffffff';
    case 'pending': return '#888';
    case 'starting': return '#FF9800';
    case 'ready': return '#4CAF50';
    case 'closing': return '#2196F3';
    case 'done': return '#000000';
    default: return '#888';
  }
};

const getTextColor = (status: ComponentStatus): string => {
  return status === 'non_runnable' ? '#fff' : '#fff';
};

const ComponentNode: React.FC<ComponentNodeProps> = ({ data }) => {
  const { component, onClick } = data;
    const backgroundColor = getStatusColor(component);
  const textColor = getTextColor(component.status);
  const borderWidth = Math.max(1, Math.round((component.delay || 0) / 1e9));
    const borderColor = (component.start_error || component.done_error) ? '#F44336' : '#333';

  return (
    <div
        className="nodrag nopan"
      style={{
        background: backgroundColor,
        color: textColor,
        padding: '8px 12px',
        borderRadius: '4px',
        border: `${borderWidth}px solid ${borderColor}`,
        fontSize: '12px',
        fontFamily: 'Courier New, monospace',
        cursor: 'pointer',
          boxSizing: 'border-box',
          width: 150,
          height: 64,
        textAlign: 'center',
        boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
          pointerEvents: 'all',
          overflow: 'hidden',
      }}
      onClick={() => onClick(component)}
    >
      <Handle type="target" position={Position.Top} />
        <div
            style={{
                fontWeight: 'bold',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
            }}
            title={component.name}
        >
            {component.name}
        </div>
        <div
            style={{
                height: 20,
                overflow: 'hidden',
            }}
        >
        {component.status === 'done' && (
            <div
                style={{
                    fontSize: '10px',
                    color: '#b71c1c',
                    background: '#ffdde0',
                    marginTop: '4px',
                    display: 'block',
                    padding: '2px 6px',
                    borderRadius: '3px',
                    maxHeight: '100%',
                    overflow: 'hidden',
                    wordBreak: 'break-word',
                    whiteSpace: 'normal',
                }}
                title={component.done_error ?? 'nil'}
            >
                {component.done_error ?? 'nil'}
            </div>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
};

export default memo(ComponentNode);
