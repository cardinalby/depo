import React from 'react';
import { ComponentStatus } from '../types/api';

interface StatusIndicatorProps {
  status: ComponentStatus;
    runnerError?: string;
}

const StatusIndicator: React.FC<StatusIndicatorProps> = ({status, runnerError}) => {
    const showResult = status === 'done';
    const hasError = !!(runnerError && runnerError.length > 0);
    const resultText = hasError ? runnerError : 'nil';

    return (
    <div className="status-line">
        <div>
            <b>Status:</b> <span className={`status-state ${status}`}>{status}</span>
        </div>
        {showResult && (
            <div className="result-line">
                <b>result:</b>{' '}
                <span className={`result-value ${hasError ? 'error' : 'ok'}`}>{resultText}</span>
            </div>
        )}
    </div>
  );
};

export default StatusIndicator;
