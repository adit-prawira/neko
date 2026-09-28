use super::resource::ScoredVector;

pub struct PartialTopK {
    pub scored_vectors: Vec<ScoredVector>,
}

impl PartialTopK {
    pub fn new() -> Self {
        Self { scored_vectors: Vec::new() }
    }

    // Consider to add new scored vector, and drop the worst one
    // if it is over top-K
    pub fn consider(&mut self, id: String, score: f32, top_k: usize, should_sort_descending: bool) {
        self.scored_vectors.push(ScoredVector { id, score });
        let should_consider_adding_scored_vector = self.scored_vectors.len() > top_k;
        if !should_consider_adding_scored_vector {
            return;
        }

        let worst_index = self
            .scored_vectors
            .iter()
            .enumerate()
            .max_by(|a, b| {
                let compare = a.1.score.total_cmp(&b.1.score);
                if should_sort_descending { compare.reverse() } else { compare }
            })
            .map(|(index, _)| index);

        let Some(worst_index) = worst_index else { return };
        self.scored_vectors.swap_remove(worst_index);
    }

    // take and move ownership of scored vectors
    pub fn take_scored_vectors(self) -> Vec<ScoredVector> {
        self.scored_vectors
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn given_new_partial_then_scored_vectors_is_empty() {
        let partial = PartialTopK::new();
        assert!(partial.scored_vectors.is_empty());
    }

    #[test]
    fn given_consider_within_top_k_then_keeps_all_entries() {
        let mut partial = PartialTopK::new();
        partial.consider("a".into(), 10.0, 5, false);
        partial.consider("b".into(), 20.0, 5, false);
        partial.consider("c".into(), 5.0, 5, false);

        let mut ids: Vec<&str> = partial.scored_vectors.iter().map(|v| v.id.as_str()).collect();
        ids.sort_unstable();
        assert_eq!(ids, vec!["a", "b", "c"]);
    }

    #[test]
    fn given_consider_ascending_past_top_k_then_drops_largest_score() {
        // For L2/Cosine the worst (to drop) is the largest score.
        // Push 5 entries against top_k=3, the two largest must be evicted.
        let mut partial = PartialTopK::new();
        let entries = [("a", 1.0_f32), ("b", 5.0_f32), ("c", 2.0_f32), ("d", 9.0_f32), ("e", 3.0_f32)];
        for (id, score) in entries {
            partial.consider(id.into(), score, 3, false);
        }

        // Order after eviction is unspecified; assert surviving ids as a set.
        let mut ids: Vec<&str> = partial.scored_vectors.iter().map(|v| v.id.as_str()).collect();
        ids.sort_unstable();
        assert_eq!(ids, vec!["a", "c", "e"], "two largest (9, 5) must be dropped, keep a=1, c=2, e=3");
    }

    #[test]
    fn given_consider_descending_past_top_k_then_drops_smallest_score() {
        // For Dot the worst (to drop) is the smallest score (since stored negated).
        let mut partial = PartialTopK::new();
        let entries = [
            ("a", -1.0_f32), // dot 1
            ("b", -5.0_f32), // dot 5
            ("c", -2.0_f32), // dot 2
            ("d", -9.0_f32), // dot 9
            ("e", -3.0_f32), // dot 3
        ];
        for (id, score) in entries {
            partial.consider(id.into(), score, 3, true);
        }

        let mut ids: Vec<&str> = partial.scored_vectors.iter().map(|v| v.id.as_str()).collect();
        ids.sort_unstable();
        assert_eq!(ids, vec!["a", "c", "e"], "drop -9 (dot 9 lost) and -5 (dot 5 lost); keep dot 1/2/3 → a, c, e");
    }

    #[test]
    fn given_take_scored_vectors_then_returns_inner_vec_and_consumes_partial() {
        let mut partial = PartialTopK::new();
        partial.consider("a".into(), 1.0, 5, false);
        partial.consider("b".into(), 2.0, 5, false);

        let taken = partial.take_scored_vectors();
        let ids: Vec<&str> = taken.iter().map(|v| v.id.as_str()).collect();
        assert_eq!(ids, vec!["a", "b"]);
    }
}
