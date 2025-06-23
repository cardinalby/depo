import React from 'react';

const Legend: React.FC = () => {
    return (
        <div className="legend">
            <div><b>Legend:</b></div>
            <div className="legend-item"><span className="legend-color" style={{background: '#888'}}/> Pending</div>
            <div className="legend-item"><span className="legend-color" style={{background: '#FF9800'}}/> Starting</div>
            <div className="legend-item"><span className="legend-color" style={{background: '#4CAF50'}}/> Ready</div>
            <div className="legend-item"><span className="legend-color" style={{background: '#2196F3'}}/> Closing</div>
            <div className="legend-item"><span className="legend-color" style={{background: '#000'}}/> Done</div>
        </div>
    );
};

export default Legend;
