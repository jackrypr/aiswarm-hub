import React, { useState, useEffect } from 'react';

/**
 * SwarmConsensus displays the aggregated AI agent predictions for a market
 */
const SwarmConsensus = ({ marketId }) => {
  const [consensus, setConsensus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetchSwarmConsensus();
  }, [marketId]);

  const fetchSwarmConsensus = async () => {
    try {
      setLoading(true);
      const response = await fetch(`/v0/markets/${marketId}/swarm`);
      if (!response.ok) {
        throw new Error('Failed to fetch swarm consensus');
      }
      const data = await response.json();
      setConsensus(data);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="swarm-consensus loading">
        <div className="spinner"></div>
        <p>Loading AI Swarm predictions...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="swarm-consensus error">
        <p>⚠️ {error}</p>
      </div>
    );
  }

  if (!consensus || consensus.totalAgents === 0) {
    return (
      <div className="swarm-consensus empty">
        <div className="swarm-header">
          <h3>🤖 AI Swarm Prediction</h3>
        </div>
        <p className="no-data">No AI agents have predicted on this market yet.</p>
      </div>
    );
  }

  const yesPct = (consensus.consensusProbability * 100).toFixed(1);
  const noPct = (100 - consensus.consensusProbability * 100).toFixed(1);

  return (
    <div className="swarm-consensus">
      <div className="swarm-header">
        <h3>🤖 AI Swarm Prediction</h3>
        <span className="agent-count">{consensus.totalAgents} agents</span>
      </div>

      {/* Main probability display */}
      <div className="consensus-probability">
        <div className="prob-bar">
          <div 
            className="prob-yes" 
            style={{ width: `${yesPct}%` }}
          >
            {yesPct}% YES
          </div>
          <div 
            className="prob-no" 
            style={{ width: `${noPct}%` }}
          >
            {noPct}% NO
          </div>
        </div>
      </div>

      {/* Stats grid */}
      <div className="swarm-stats">
        <div className="stat">
          <span className="label">Total Bets</span>
          <span className="value">{consensus.totalBets}</span>
        </div>
        <div className="stat">
          <span className="label">Total Wagered</span>
          <span className="value">{consensus.totalWagered?.toLocaleString()}</span>
        </div>
        <div className="stat">
          <span className="label">Avg Confidence</span>
          <span className="value">{(consensus.averageConfidence * 100).toFixed(0)}%</span>
        </div>
        <div className="stat">
          <span className="label">Avg Reputation</span>
          <span className="value">{(consensus.averageReputation * 100).toFixed(0)}%</span>
        </div>
      </div>

      {/* Breakdown */}
      <div className="breakdown">
        <h4>Vote Breakdown</h4>
        <div className="breakdown-row">
          <span className="outcome yes">YES</span>
          <span className="count">{consensus.breakdown?.yesCount || 0} votes</span>
          <span className="amount">{consensus.breakdown?.yesAmount?.toLocaleString() || 0} wagered</span>
        </div>
        <div className="breakdown-row">
          <span className="outcome no">NO</span>
          <span className="count">{consensus.breakdown?.noCount || 0} votes</span>
          <span className="amount">{consensus.breakdown?.noAmount?.toLocaleString() || 0} wagered</span>
        </div>
      </div>

      {/* Top Predictors */}
      {consensus.topPredictors && consensus.topPredictors.length > 0 && (
        <div className="top-predictors">
          <h4>Top Predictors</h4>
          <div className="predictor-list">
            {consensus.topPredictors.slice(0, 5).map((predictor, idx) => (
              <div key={idx} className="predictor">
                <div className="predictor-header">
                  <span className="rank">#{idx + 1}</span>
                  <span className="name">{predictor.agentName}</span>
                  <span className={`outcome ${predictor.outcome}`}>
                    {predictor.outcome.toUpperCase()}
                  </span>
                </div>
                <div className="predictor-stats">
                  <div className="confidence-bar">
                    <div 
                      className="fill" 
                      style={{ width: `${predictor.confidence * 100}%` }}
                    />
                    <span>{(predictor.confidence * 100).toFixed(0)}% conf</span>
                  </div>
                  <span className="reputation">
                    ⭐ {(predictor.reputation * 100).toFixed(0)}%
                  </span>
                  <span className="amount">
                    {predictor.amount?.toLocaleString()}
                  </span>
                </div>
                {predictor.reasoning && (
                  <div className="reasoning">
                    💭 {predictor.reasoning}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      <style jsx>{`
        .swarm-consensus {
          background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
          border-radius: 12px;
          padding: 20px;
          margin: 16px 0;
          color: #fff;
        }

        .swarm-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          margin-bottom: 16px;
        }

        .swarm-header h3 {
          margin: 0;
          font-size: 1.25rem;
        }

        .agent-count {
          background: rgba(255,255,255,0.1);
          padding: 4px 12px;
          border-radius: 20px;
          font-size: 0.875rem;
        }

        .consensus-probability {
          margin-bottom: 20px;
        }

        .prob-bar {
          display: flex;
          height: 40px;
          border-radius: 8px;
          overflow: hidden;
          font-weight: 600;
        }

        .prob-yes {
          background: linear-gradient(90deg, #10b981 0%, #34d399 100%);
          display: flex;
          align-items: center;
          justify-content: center;
          color: #fff;
          transition: width 0.5s ease;
        }

        .prob-no {
          background: linear-gradient(90deg, #ef4444 0%, #f87171 100%);
          display: flex;
          align-items: center;
          justify-content: center;
          color: #fff;
          transition: width 0.5s ease;
        }

        .swarm-stats {
          display: grid;
          grid-template-columns: repeat(4, 1fr);
          gap: 12px;
          margin-bottom: 20px;
        }

        .stat {
          background: rgba(255,255,255,0.05);
          padding: 12px;
          border-radius: 8px;
          text-align: center;
        }

        .stat .label {
          display: block;
          font-size: 0.75rem;
          color: #9ca3af;
          margin-bottom: 4px;
        }

        .stat .value {
          font-size: 1.125rem;
          font-weight: 600;
        }

        .breakdown {
          background: rgba(255,255,255,0.05);
          border-radius: 8px;
          padding: 16px;
          margin-bottom: 20px;
        }

        .breakdown h4 {
          margin: 0 0 12px 0;
          font-size: 0.875rem;
          color: #9ca3af;
        }

        .breakdown-row {
          display: flex;
          justify-content: space-between;
          padding: 8px 0;
          border-bottom: 1px solid rgba(255,255,255,0.1);
        }

        .breakdown-row:last-child {
          border-bottom: none;
        }

        .outcome {
          font-weight: 600;
          padding: 2px 8px;
          border-radius: 4px;
        }

        .outcome.yes {
          background: rgba(16, 185, 129, 0.2);
          color: #10b981;
        }

        .outcome.no {
          background: rgba(239, 68, 68, 0.2);
          color: #ef4444;
        }

        .top-predictors h4 {
          margin: 0 0 12px 0;
          font-size: 0.875rem;
          color: #9ca3af;
        }

        .predictor {
          background: rgba(255,255,255,0.05);
          border-radius: 8px;
          padding: 12px;
          margin-bottom: 8px;
        }

        .predictor-header {
          display: flex;
          align-items: center;
          gap: 8px;
          margin-bottom: 8px;
        }

        .predictor-header .rank {
          color: #9ca3af;
          font-size: 0.875rem;
        }

        .predictor-header .name {
          font-weight: 600;
          flex: 1;
        }

        .predictor-stats {
          display: flex;
          align-items: center;
          gap: 12px;
          font-size: 0.875rem;
        }

        .confidence-bar {
          flex: 1;
          background: rgba(255,255,255,0.1);
          height: 20px;
          border-radius: 4px;
          position: relative;
          overflow: hidden;
        }

        .confidence-bar .fill {
          position: absolute;
          left: 0;
          top: 0;
          height: 100%;
          background: linear-gradient(90deg, #6366f1 0%, #8b5cf6 100%);
        }

        .confidence-bar span {
          position: relative;
          z-index: 1;
          display: flex;
          align-items: center;
          justify-content: center;
          height: 100%;
          font-size: 0.75rem;
        }

        .reasoning {
          margin-top: 8px;
          padding-top: 8px;
          border-top: 1px solid rgba(255,255,255,0.1);
          font-size: 0.875rem;
          color: #9ca3af;
          font-style: italic;
        }

        .loading, .error, .empty {
          text-align: center;
          padding: 40px 20px;
        }

        .no-data {
          color: #9ca3af;
        }

        @media (max-width: 640px) {
          .swarm-stats {
            grid-template-columns: repeat(2, 1fr);
          }
        }
      `}</style>
    </div>
  );
};

export default SwarmConsensus;
