import React, {useCallback, useEffect, useState} from 'react';
import { Component } from '../types/api';
import { apiService } from '../services/api';

interface ComponentModalProps {
  component: Component;
  onClose: () => void;
  onUpdate: () => void;
}

const ComponentModal: React.FC<ComponentModalProps> = ({ component, onClose, onUpdate }) => {
  const [delay, setDelay] = useState(Math.round(component.delay / 1e9) || 0);
  const [error, setError] = useState(component.start_error || '');

    const handleClose = useCallback(() => {
    try {
      apiService.updateComponent({
        component_id: component.id,
        delay_ms: delay * 1000,
        start_error: error.trim() === '' ? '' : error.trim(),
      });
      onUpdate();
    } catch (err) {
      console.error('Failed to update component:', err);
    } finally {
        onClose();
    }
  }, [component.id, delay, error, onUpdate, onClose]);

    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                handleClose();
            }
        };
        document.addEventListener('keydown', onKeyDown);
        return () => document.removeEventListener('keydown', onKeyDown);
    }, [handleClose]);

    const handleStop = async (withError: boolean) => {

        try {
        apiService.stopComponent({
            component_id: component.id,
            with_error: withError
        });
        // After stopping, also perform update on close
        handleClose();
    } catch (err) {
        console.error('Failed to stop component:', err);
    }
  };

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
        handleClose();
    }
  };

  return (
      <div className="component-modal" onMouseDown={handleBackdropClick} role="dialog" aria-modal="true">
      <div className="component-modal-content">
        <h3>Component: {component.name}</h3>
        <p><strong>ID:</strong> {component.id}</p>
        <p><strong>Status:</strong> {component.status}</p>

        <div className="form-group">
          <label htmlFor="delay">Delay (seconds):</label>
          <input
            id="delay"
            type="number"
            value={delay}
            onChange={(e) => setDelay(parseInt(e.target.value) || 0)}
            min="0"
          />
        </div>

        <div className="form-group">
          <label htmlFor="error">Start error (empty for no error):</label>
          <input
            id="error"
            type="text"
            value={error}
            onChange={(e) => setError(e.target.value)}
            placeholder="Leave empty for no error"
          />
        </div>

          <div className="finish-run-group">
              <strong>Finish Run</strong>
              <div className="modal-buttons">
                  <button onClick={() => handleStop(true)} className="danger">
                      With some_err
                  </button>
                  <button onClick={() => handleStop(false)}>
                      With nil error
                  </button>
          </div>
        </div>
          <div className="hint">Press Esc or click outside to close</div>
      </div>
    </div>
  );
};

export default ComponentModal;
