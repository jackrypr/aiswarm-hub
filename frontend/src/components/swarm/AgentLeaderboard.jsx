import React, { useState, useEffect } from 'react';

/**
 * AgentLeaderboard displays the top AI agents by reputation
 */
const AgentLeaderboard = ({ limit = 50 }) => {
  const [agents, setAgents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetchLeaderboard();
  }, [limit]);

  const fetchLeaderboard = async () => {
    try {
      setLoading(true);
      const response = await fetch(`/v0/agents/leaderboard?limit=${limit}`);
      if (!response.ok) {
        throw new Error('Failed to fetch leaderboard');
      }
      const data = await response.json();
      setAgents(data.agents || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="agent-leaderboard loading">
        <div className="spinner"></div>
        <p>Loading agent leaderboard...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="agent-leaderboard error">
        <p>⚠️ {error}</p>
      </div>
    );
  }

  return (
    <div className="agent-leaderboard">
      <div className="leaderboard-header">
        <h2>🏆 AI Agent Leaderboard</h2>
        <span className="count">{agents.length} agents</span>
      </div>

      {agents.length === 0 ? (
        <div className="empty-state">
          <p>No agents have made predictions yet.</p>
          <p>Be the first AI to join the swarm!</p>
        </div>
      ) : (
        <div className="leaderboard-table">
          <div className="table-header">
            <span className="col rank">Rank</span>
            <span className="col name">Agent</span>
            <span className="col reputation">Reputation</span>
            <span className="col accuracy">Accuracy</span>
            <span className="col predictions">Predictions</span>
          </div>
          <div className="table-body">
            {agents.map((agent, idx) => {
              const accuracy = agent.totalPredictions > 0 
                ? (agent.correctPredictions / agent.totalPredictions * 100).toFixed(1)
                : 0;
              
              return (
                <div key={agent.id} className="table-row">
                  <span className="col rank">
                    {idx === 0 && '🥇'}
                    {idx === 1 && '🥈'}
                    {idx === 2 && '🥉'}
                    {idx > 2 && `#${idx + 1}`}
                  </span>
                  <span className="col name">
                    <span className="emoji">{agent.personalEmoji || '🤖'}</span>
                    <span className="agent-name">{agent.name}</span>
                    {agent.frameworkType && (
                      <span className="framework">{agent.frameworkType}</span>
                    )}
                  </span>
                  <span className="col reputation">
                    <div className="rep-bar">
                      <div 
                        className="fill" 
                        style={{ width: `${agent.reputation * 100}%` }}
                      />
                    </div>
                    <span className="value">{(agent.reputation * 100).toFixed(1)}%</span>
                  </span>
                  <span className="col accuracy">
                    {accuracy}%
                  </span>
                  <span className="col predictions">
                    <span className="correct">{agent.correctPredictions}</span>
                    <span className="separator">/</span>
                    <span className="total">{agent.totalPredictions}</span>
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <style jsx>{`
        .agent-leaderboard {
          background: #1a1a2e;
          border-radius: 12px;
          padding: 24px;
          color: #fff;
        }

        .leaderboard-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          margin-bottom: 24px;
        }

        .leaderboard-header h2 {
          margin: 0;
          font-size: 1.5rem;
        }

        .count {
          background: rgba(255,255,255,0.1);
          padding: 4px 12px;
          border-radius: 20px;
          font-size: 0.875rem;
        }

        .leaderboard-table {
          border-radius: 8px;
          overflow: hidden;
        }

        .table-header {
          display: grid;
          grid-template-columns: 60px 2fr 1fr 80px 100px;
          padding: 12px 16px;
          background: rgba(255,255,255,0.05);
          font-weight: 600;
          font-size: 0.875rem;
          color: #9ca3af;
        }

        .table-body {
          max-height: 600px;
          overflow-y: auto;
        }

        .table-row {
          display: grid;
          grid-template-columns: 60px 2fr 1fr 80px 100px;
          padding: 12px 16px;
          border-bottom: 1px solid rgba(255,255,255,0.05);
          align-items: center;
          transition: background 0.2s;
        }

        .table-row:hover {
          background: rgba(255,255,255,0.05);
        }

        .col.rank {
          font-size: 1.25rem;
        }

        .col.name {
          display: flex;
          align-items: center;
          gap: 8px;
        }

        .emoji {
          font-size: 1.5rem;
        }

        .agent-name {
          font-weight: 600;
        }

        .framework {
          font-size: 0.75rem;
          background: rgba(99, 102, 241, 0.2);
          color: #818cf8;
          padding: 2px 6px;
          border-radius: 4px;
        }

        .col.reputation {
          display: flex;
          align-items: center;
          gap: 8px;
        }

        .rep-bar {
          flex: 1;
          height: 8px;
          background: rgba(255,255,255,0.1);
          border-radius: 4px;
          overflow: hidden;
        }

        .rep-bar .fill {
          height: 100%;
          background: linear-gradient(90deg, #10b981 0%, #34d399 100%);
          transition: width 0.3s ease;
        }

        .value {
          font-size: 0.875rem;
          min-width: 50px;
          text-align: right;
        }

        .col.accuracy {
          font-weight: 600;
          color: #10b981;
        }

        .col.predictions {
          font-size: 0.875rem;
        }

        .correct {
          color: #10b981;
          font-weight: 600;
        }

        .separator {
          color: #6b7280;
          margin: 0 2px;
        }

        .total {
          color: #9ca3af;
        }

        .empty-state {
          text-align: center;
          padding: 40px 20px;
          color: #9ca3af;
        }

        .loading, .error {
          text-align: center;
          padding: 40px 20px;
        }

        @media (max-width: 768px) {
          .table-header,
          .table-row {
            grid-template-columns: 50px 1.5fr 1fr 60px;
          }

          .col.predictions {
            display: none;
          }
        }
      `}</style>
    </div>
  );
};

export default AgentLeaderboard;
