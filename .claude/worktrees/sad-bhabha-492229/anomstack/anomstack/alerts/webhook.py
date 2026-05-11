"""
Helper functions to send webhook alerts.
"""

import json
import os
import requests
import pandas as pd
from dagster import get_dagster_logger

def send_alert_webhook(
    title: str,
    message: str,
    df: pd.DataFrame = None,
    metric_name: str = None,
    score_col: str = None,
    threshold: float = None,
) -> bool:
    """
    Sends an alert via a generic webhook POST request.
    
    Args:
        title (str): Title of the alert.
        message (str): Details of the alert.
        df (pd.DataFrame): Dataframe containing the anomaly data.
        metric_name (str): Name of the metric.
        score_col (str): Score column name.
        threshold (float): Configured threshold.
        
    Returns:
        bool: True if successful, False otherwise.
    """
    logger = get_dagster_logger()
    webhook_url = os.environ.get("ANOMSTACK_WEBHOOK_URL")
    if not webhook_url:
        logger.warning("ANOMSTACK_WEBHOOK_URL not set. Cannot send webhook alert.")
        return False
        
    payload = {
        "title": title,
        "message": message,
        "metric_name": metric_name,
        "threshold": threshold,
    }
    
    # Optionally include recent anomaly points if df is provided
    if df is not None and not df.empty:
        # Extract the most recent anomalous row
        try:
            recent_anomalies = df[df[score_col] > threshold].tail(1)
            if not recent_anomalies.empty:
                payload["anomaly_details"] = json.loads(recent_anomalies.to_json(orient="records"))[0]
        except Exception as e:
            logger.error(f"Failed to extract anomaly details for webhook: {e}")

    try:
        response = requests.post(
            webhook_url,
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=10
        )
        response.raise_for_status()
        logger.info(f"Successfully sent webhook alert to {webhook_url}")
        return True
    except Exception as e:
        logger.error(f"Failed to send webhook alert: {e}")
        return False

def send_df_webhook(title: str, df: pd.DataFrame) -> bool:
    """
    Sends a dataframe dump via webhook.
    """
    return send_alert_webhook(title=title, message="Dataframe dump", df=df)
