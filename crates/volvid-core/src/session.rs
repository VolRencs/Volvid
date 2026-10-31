use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct SessionItem {
    pub label: String,
    pub url: String,
    pub ok: bool,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Session {
    pub success: usize,
    pub failed: usize,
    pub items: Vec<SessionItem>,
}

impl Session {
    pub fn record(&mut self, label: impl Into<String>, url: impl Into<String>, ok: bool) {
        if ok {
            self.success += 1;
        } else {
            self.failed += 1;
        }
        self.items.push(SessionItem {
            label: label.into(),
            url: url.into(),
            ok,
        });
    }
}
