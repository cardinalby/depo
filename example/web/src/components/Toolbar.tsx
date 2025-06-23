import React from 'react';

interface ToolbarProps {
    isIdle: boolean;
    shutDownOnNilRunResult: boolean;
    onStart: () => void;
    onStop: () => void;
    onReset: (shutDownOnNilRunResult: boolean) => void;
    statusLabel?: React.ReactNode;
}

const Toolbar: React.FC<ToolbarProps> = ({
                                             isIdle,
                                             shutDownOnNilRunResult,
                                             onStart,
                                             onStop,
                                             onReset,
                                             statusLabel,
                                         }) => {

    return (
        <div className="toolbar">
            <div style={{display: 'flex', gap: '8px', marginBottom: 8}}>
                {isIdle ? (
                    <button style={{background: 'black', color: 'white'}} onClick={onStart}>Start</button>
                ) : (
                    <button style={{background: 'black', color: 'white'}}
                            onClick={() => onReset(shutDownOnNilRunResult)}>Reset</button>
                )}
                <button style={{background: 'black', color: 'white'}} onClick={onStop}>Stop</button>
            </div>
            <div style={{
                marginBottom: 8,
                background: 'white',
                padding: '4px 8px',
                borderRadius: 4,
                width: 'fit-content'
            }}>
                <label>
                    <input
                        type="checkbox"
                        checked={shutDownOnNilRunResult}
                        onChange={e => onReset(e.target.checked)}
                    />
                    Shutdown on nil run result
                </label>
            </div>
            <div>{statusLabel}</div>
        </div>
    );
};

export default Toolbar;
