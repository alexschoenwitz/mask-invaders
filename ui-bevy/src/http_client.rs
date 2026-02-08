use crate::data::{GetStateHistoryResponse, GetStateResponse, ListGamesResponse, State};
use std::time::Duration;

pub struct HttpClient {
    client: reqwest::blocking::Client,
    base_url: String,
}

impl HttpClient {
    pub fn new(base_url: &str) -> Self {
        Self {
            client: reqwest::blocking::Client::builder()
                .timeout(Duration::from_secs(5))
                .build()
                .expect("Failed to create HTTP client"),
            base_url: base_url.to_string(),
        }
    }

    pub fn fetch_game_list(&self) -> Result<Vec<String>, String> {
        let url = format!("{}/v1/games", self.base_url);
        let response = self
            .client
            .get(&url)
            .send()
            .map_err(|e| format!("Failed to fetch games: {}", e))?;

        if !response.status().is_success() {
            return Err(format!(
                "Server returned status {}: {}",
                response.status(),
                response.text().unwrap_or_default()
            ));
        }

        let body = response
            .text()
            .map_err(|e| format!("Failed to read response: {}", e))?;

        let games_response: ListGamesResponse =
            serde_json::from_str(&body).map_err(|e| format!("Failed to parse JSON: {}", e))?;

        Ok(games_response.game_ids)
    }

    pub fn fetch_state(&self, game_id: &str) -> Result<Option<State>, String> {
        let url = format!("{}/v1/games/{}/state", self.base_url, game_id);
        let response = self
            .client
            .get(&url)
            .send()
            .map_err(|e| format!("Failed to fetch state: {}", e))?;

        if !response.status().is_success() {
            return Err(format!(
                "Server returned status {}: {}",
                response.status(),
                response.text().unwrap_or_default()
            ));
        }

        let body = response
            .text()
            .map_err(|e| format!("Failed to read response: {}", e))?;

        let state_response: GetStateResponse =
            serde_json::from_str(&body).map_err(|e| format!("Failed to parse JSON: {}", e))?;

        Ok(state_response.state)
    }

    pub fn fetch_state_history(&self, game_id: &str) -> Result<Vec<State>, String> {
        let url = format!("{}/v1/games/{}/state:history", self.base_url, game_id);
        let response = self
            .client
            .get(&url)
            .timeout(Duration::from_secs(10))
            .send()
            .map_err(|e| format!("Failed to fetch history: {}", e))?;

        if !response.status().is_success() {
            return Err(format!(
                "Server returned status {}: {}",
                response.status(),
                response.text().unwrap_or_default()
            ));
        }

        let body = response
            .text()
            .map_err(|e| format!("Failed to read response: {}", e))?;

        let history_response: GetStateHistoryResponse =
            serde_json::from_str(&body).map_err(|e| format!("Failed to parse JSON: {}", e))?;

        Ok(history_response.states)
    }
}

pub fn load_replay_file(path: &str) -> Result<Vec<State>, String> {
    let content =
        std::fs::read_to_string(path).map_err(|e| format!("Failed to read file: {}", e))?;

    let states: Vec<State> =
        serde_json::from_str(&content).map_err(|e| format!("Failed to parse JSON: {}", e))?;

    Ok(states)
}
