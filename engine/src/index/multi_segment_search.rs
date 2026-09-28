use rayon::iter::IntoParallelRefIterator;
use rayon::prelude::*;
use std::collections::BinaryHeap;

use crate::segment::reader::SegmentReader;
use crate::segment::resource::Metric;
use crate::shared::hairball::Hairball;
use crate::shared::results::Result;

use super::partial_top_k::PartialTopK;
use super::resource::ScoredVector;

#[derive(Debug, Clone)]
pub struct MultiSegmentSearchParams<'a> {
    pub readers: &'a [SegmentReader],
    pub query: &'a [f32],
    pub top_k: usize,
    pub metric: u8,
    pub dim: u32,
}

pub struct MultiSegmentSearch;

impl MultiSegmentSearch {
    pub fn search(live_top_k: Vec<ScoredVector>, search_params: &MultiSegmentSearchParams) -> Result<Vec<ScoredVector>> {
        let should_skip_search = search_params.readers.is_empty() || search_params.top_k == 0;
        let metric = Metric::from_u8(&search_params.metric)?;

        if should_skip_search {
            let mut live = live_top_k;
            metric.sort(&mut live);
            return Ok(live);
        }

        let should_sort_descending = matches!(metric, Metric::Dot);

        // ensure head capacity to be as small as possible
        let head_capacity = search_params.top_k.max(live_top_k.len());

        // add additional heap to allow re-allocation of vector on the N'th + 1 capacity push
        let mut heap: BinaryHeap<ScoredVector> = BinaryHeap::with_capacity(head_capacity + 1);

        let partial_top_ks: Vec<PartialTopK> = search_params
            .readers
            .par_iter()
            .map(|reader| Self::partial_top_k(reader, &metric, search_params, should_sort_descending))
            .collect::<Result<Vec<_>>>()?;

        for partial_top_k in partial_top_ks {
            for scored_vector in partial_top_k.take_scored_vectors() {
                heap.push(scored_vector);
                let is_above_capacity = heap.len() > head_capacity;
                if is_above_capacity {
                    heap.pop();
                }
            }
        }

        for scored_vector in live_top_k {
            heap.push(scored_vector);
            let is_above_capacity = heap.len() > head_capacity;
            if is_above_capacity {
                heap.pop();
            }
        }

        let mut scored_vectors: Vec<ScoredVector> = heap.into_vec();
        metric.sort(&mut scored_vectors);
        Ok(scored_vectors)
    }

    fn partial_top_k(reader: &SegmentReader, metric: &Metric, search_params: &MultiSegmentSearchParams, should_sort_descending: bool) -> Result<PartialTopK> {
        if reader.dim() != search_params.dim {
            return Err(Hairball::DimMismatch);
        }

        let mut partial = PartialTopK::new();
        let vectors = reader.iter_vectors();
        let mut metadata_iter = reader.iter_metadata()?;

        for vector_result in vectors {
            let vector = vector_result?;
            let metadata = metadata_iter.next().ok_or(Hairball::CorruptedSegment)??;
            if metadata.deleted {
                continue;
            }

            let distance = metric.distance(search_params.query, vector, &search_params.dim);
            partial.consider(metadata.id, distance, search_params.top_k, should_sort_descending);
        }

        Ok(partial)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::segment::resource::VectorMetadata;
    use crate::segment::writer::SegmentWriter;
    use std::fs;
    use std::path::Path;

    fn fresh_dir(name: &str) -> std::path::PathBuf {
        let dir = std::env::temp_dir().join(format!("neko_test_multi_segment_{}", name));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    fn make_metadata(id: &str, deleted: bool) -> VectorMetadata {
        VectorMetadata {
            id: id.to_string(),
            created_at: 0,
            deleted,
            custom: String::new(),
        }
    }

    fn write_segment(directory: &Path, name: &str, dim: u32, rows: &[(&str, &[f32])]) -> std::path::PathBuf {
        let mut writer = SegmentWriter::new(directory, name, dim).unwrap();
        for (id, vector) in rows {
            writer.append(vector, &make_metadata(id, false)).unwrap();
        }
        writer.finish().unwrap();
        directory.join(name)
    }

    fn open_reader(directory: &Path) -> SegmentReader {
        SegmentReader::open_from_directory(directory).expect("segment should open with valid header")
    }

    fn ids_ordered(results: &[ScoredVector]) -> Vec<String> {
        results.iter().map(|r| r.id.clone()).collect()
    }

    #[test]
    fn given_empty_readers_and_non_empty_live_then_returns_sorted_live_top_k() {
        // Use L2 with single live entry to verify the empty-readers branch
        // routes through Metric::sort and leaves live results intact.
        let live = vec![ScoredVector { id: "far".into(), score: 9.0 }, ScoredVector { id: "near".into(), score: 1.0 }];
        let params = MultiSegmentSearchParams {
            readers: &[],
            query: &[1.0_f32, 0.0, 0.0][..],
            top_k: 5,
            metric: 0, // L2
            dim: 3,
        };
        let result = MultiSegmentSearch::search(live, &params).unwrap();
        assert_eq!(ids_ordered(&result), vec!["near", "far"]);
    }

    #[test]
    fn given_top_k_zero_then_short_circuits_to_sorted_live_only() {
        // The skip-search branch fires on top_k == 0 OR empty readers.
        // When there are no readers, the live top-K is sorted and returned
        // as-is — MultiSegmentSearch does not drop live entries because the
        // caller asked for top_k=0.
        let live = vec![ScoredVector { id: "far".into(), score: 9.0 }, ScoredVector { id: "near".into(), score: 1.0 }];
        let params = MultiSegmentSearchParams {
            readers: &[],
            query: &[1.0_f32, 0.0, 0.0][..],
            top_k: 0,
            metric: 0,
            dim: 3,
        };
        let result = MultiSegmentSearch::search(live, &params).unwrap();
        // After sort, ascending by score: near first, then far.
        assert_eq!(result.len(), 2);
        assert_eq!(result[0].id, "near");
        assert_eq!(result[1].id, "far");
    }

    #[test]
    fn given_multi_segment_l2_then_top_k_merges_across_segments() {
        // Two segments, each holds 2 vectors in 2-d.
        // Segment A: near (1,0),  far_a (10,0)
        // Segment B: mid   (5,0),  far_b (100,0)
        // Query (1,0).  Brute ground truth top-2 by L2: near (0), mid (4).
        let dir = fresh_dir("merges_across_segments");
        let seg_a_path = write_segment(&dir, "seg_a", 2, &[("near", &[1.0, 0.0][..]), ("far_a", &[10.0, 0.0][..])]);
        let seg_b_path = write_segment(&dir, "seg_b", 2, &[("mid", &[5.0, 0.0][..]), ("far_b", &[100.0, 0.0][..])]);
        let readers = vec![open_reader(&seg_a_path), open_reader(&seg_b_path)];

        let params = MultiSegmentSearchParams {
            readers: &readers,
            query: &[1.0_f32, 0.0][..],
            top_k: 2,
            metric: 0,
            dim: 2,
        };
        let live: Vec<ScoredVector> = Vec::new();
        let result = MultiSegmentSearch::search(live, &params).unwrap();

        // Set comparison: top-K by score may contain any of the four ids;
        // assert the two smallest L2 distances survive.
        let mut scored: Vec<(&str, f32)> = result.iter().map(|r| (r.id.as_str(), r.score)).collect();
        scored.sort_by(|a, b| a.1.total_cmp(&b.1));
        let (first_id, first_score) = scored[0];
        let (second_id, second_score) = scored[1];
        assert_eq!(first_id, "near", "lowest L2 must be 'near'");
        assert!(first_score < 0.001, "near's L2 should be ~0, got {}", first_score);
        assert_eq!(second_id, "mid", "second lowest L2 must be 'mid'");
        assert!((second_score - 4.0).abs() < 0.001, "mid's L2 should be ~4, got {}", second_score);
    }

    #[test]
    fn given_segment_with_mismatched_dim_then_returns_dim_mismatch() {
        let dir = fresh_dir("dim_mismatch");
        // Segment is built with dim=2.
        let seg_path = write_segment(&dir, "seg", 2, &[("near", &[1.0, 0.0][..]), ("far", &[10.0, 0.0][..])]);
        // Params claim dim=3.
        let readers = vec![open_reader(&seg_path)];
        let params = MultiSegmentSearchParams {
            readers: &readers,
            query: &[1.0_f32, 0.0, 0.0][..],
            top_k: 5,
            metric: 0,
            dim: 3,
        };
        let result = MultiSegmentSearch::search(Vec::new(), &params);
        assert!(matches!(result, Err(Hairball::DimMismatch)));
    }

    #[test]
    fn given_single_segment_then_returns_top_k_from_that_segment_only() {
        // Regression guard: the multi-segment path with N=1 must produce the
        // same top-K as if we had scanned one segment linearly.
        let dir = fresh_dir("single_segment");
        let seg_path = write_segment(
            &dir,
            "seg",
            2,
            &[
                ("near", &[1.0, 0.0][..]),
                ("mid", &[5.0, 0.0][..]),
                ("far", &[10.0, 0.0][..]),
                ("farthest", &[100.0, 0.0][..]),
            ],
        );
        let readers = vec![open_reader(&seg_path)];

        let params = MultiSegmentSearchParams {
            readers: &readers,
            query: &[1.0_f32, 0.0][..],
            top_k: 2,
            metric: 0,
            dim: 2,
        };
        let result = MultiSegmentSearch::search(Vec::new(), &params).unwrap();
        assert_eq!(result.len(), 2);
        let mut scored: Vec<(&str, f32)> = result.iter().map(|r| (r.id.as_str(), r.score)).collect();
        scored.sort_by(|a, b| a.1.total_cmp(&b.1));
        assert_eq!(scored[0].0, "near");
        assert_eq!(scored[1].0, "mid");
    }

    #[test]
    fn given_live_top_k_merged_with_segments_then_global_top_k_includes_best_from_both() {
        // Live source carries one candidate; segments carry others.
        // Top-1 by L2 against [1, 0]: live "alpha" has score 100 (far);
        // segments contain "near" (1, 0) -> score 0. Result must be "near".
        let dir = fresh_dir("live_plus_segments");
        let seg_path = write_segment(&dir, "seg", 2, &[("near", &[1.0, 0.0][..]), ("beta", &[50.0, 0.0][..])]);
        let readers = vec![open_reader(&seg_path)];

        let live = vec![ScoredVector { id: "alpha".into(), score: 100.0 }];

        let params = MultiSegmentSearchParams {
            readers: &readers,
            query: &[1.0_f32, 0.0][..],
            top_k: 1,
            metric: 0,
            dim: 2,
        };
        let result = MultiSegmentSearch::search(live, &params).unwrap();
        assert_eq!(result.len(), 1);
        assert_eq!(result[0].id, "near", "segment winner must beat the live entry");
    }

    #[test]
    fn given_two_segments_under_rayon_then_top_k_still_includes_best_from_each_segment() {
        // Regression guard for the par_iter call site: ensures switching
        // .iter() to .par_iter() in MultiSegmentSearch::search did not change
        // the result set. Worker-count itself is hardware-dependent and is
        // not asserted here — see the documented test gap below.
        let dir = fresh_dir("par_iter_regression");
        let seg_a_path = write_segment(&dir, "a", 2, &[("near", &[1.0, 0.0][..]), ("far_a", &[10.0, 0.0][..])]);
        let seg_b_path = write_segment(&dir, "b", 2, &[("mid", &[5.0, 0.0][..]), ("far_b", &[100.0, 0.0][..])]);
        let readers = vec![open_reader(&seg_a_path), open_reader(&seg_b_path)];
        let params = MultiSegmentSearchParams {
            readers: &readers,
            query: &[1.0_f32, 0.0][..],
            top_k: 2,
            metric: 0,
            dim: 2,
        };
        let result = MultiSegmentSearch::search(Vec::new(), &params).unwrap();
        let mut scored: Vec<(&str, f32)> = result.iter().map(|r| (r.id.as_str(), r.score)).collect();
        scored.sort_by(|a, b| a.1.total_cmp(&b.1));
        assert_eq!(scored[0].0, "near");
        assert_eq!(scored[1].0, "mid");
    }
}
