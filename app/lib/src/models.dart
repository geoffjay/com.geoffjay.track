/// Typed models for the /api/v1 JSON contract.
///
/// Field names and semantics mirror the Go structs in internal/fitness —
/// lists arrive as `{"data": [...]}`, single objects bare, and errors as
/// `{"error": "...", "field": "..."}` (field only on 400s).
library;

/// Parses an RFC3339 timestamp; returns null on absence/malformed input.
DateTime? _tryTime(Object? v) {
  if (v is! String || v.isEmpty) return null;
  return DateTime.tryParse(v)?.toUtc();
}

/// Formats a timestamp the way the API accepts (RFC3339, UTC).
String apiTime(DateTime t) => t.toUtc().toIso8601String();

class Metric {
  final int id;
  final String name;
  final String label;
  final String unit;
  final String category;
  final bool isSystem;
  final DateTime? createdAt;

  const Metric({
    required this.id,
    required this.name,
    required this.label,
    required this.unit,
    required this.category,
    required this.isSystem,
    this.createdAt,
  });

  factory Metric.fromJson(Map<String, dynamic> j) => Metric(
        id: (j['id'] as num).toInt(),
        name: j['name'] as String? ?? '',
        label: j['label'] as String? ?? '',
        unit: j['unit'] as String? ?? '',
        category: j['category'] as String? ?? '',
        isSystem: j['is_system'] as bool? ?? false,
        createdAt: _tryTime(j['created_at']),
      );
}

class Measurement {
  final int id;
  final int metricId;
  final String metricName;
  final String metricUnit;
  final double value;
  final DateTime? measuredAt;
  final String note;
  final DateTime? createdAt;

  const Measurement({
    required this.id,
    required this.metricId,
    required this.metricName,
    required this.metricUnit,
    required this.value,
    this.measuredAt,
    this.note = '',
    this.createdAt,
  });

  factory Measurement.fromJson(Map<String, dynamic> j) => Measurement(
        id: (j['id'] as num).toInt(),
        metricId: (j['metric_id'] as num).toInt(),
        metricName: j['metric_name'] as String? ?? '',
        metricUnit: j['metric_unit'] as String? ?? '',
        value: (j['value'] as num).toDouble(),
        measuredAt: _tryTime(j['measured_at']),
        note: j['note'] as String? ?? '',
        createdAt: _tryTime(j['created_at']),
      );
}

class Food {
  final int id;
  final String name;
  final String brand;
  final double servingSize;
  final String servingUnit;
  final double calories;
  final double protein;
  final double carbs;
  final double fat;
  final bool isSystem;
  final DateTime? createdAt;

  const Food({
    required this.id,
    required this.name,
    required this.servingSize,
    required this.servingUnit,
    required this.calories,
    required this.protein,
    required this.carbs,
    required this.fat,
    this.brand = '',
    this.isSystem = false,
    this.createdAt,
  });

  factory Food.fromJson(Map<String, dynamic> j) => Food(
        id: (j['id'] as num).toInt(),
        name: j['name'] as String? ?? '',
        brand: j['brand'] as String? ?? '',
        servingSize: (j['serving_size'] as num).toDouble(),
        servingUnit: j['serving_unit'] as String? ?? '',
        calories: (j['calories'] as num).toDouble(),
        protein: (j['protein'] as num).toDouble(),
        carbs: (j['carbs'] as num).toDouble(),
        fat: (j['fat'] as num).toDouble(),
        isSystem: j['is_system'] as bool? ?? false,
        createdAt: _tryTime(j['created_at']),
      );

  /// One-line serving description, e.g. "100 g" or "1 slice".
  String get servingLabel {
    final size = servingSize == servingSize.roundToDouble()
        ? servingSize.toInt().toString()
        : servingSize.toString();
    return '$size $servingUnit';
  }
}

class MealItem {
  final int id;
  final int mealId;
  final int? foodId;
  final String label;
  final double quantity;
  final double calories;
  final double protein;
  final double carbs;
  final double fat;
  final int position;

  const MealItem({
    required this.id,
    required this.mealId,
    required this.label,
    required this.quantity,
    required this.calories,
    required this.protein,
    required this.carbs,
    required this.fat,
    this.foodId,
    this.position = 0,
  });

  factory MealItem.fromJson(Map<String, dynamic> j) => MealItem(
        id: (j['id'] as num).toInt(),
        mealId: (j['meal_id'] as num).toInt(),
        foodId: j['food_id'] == null ? null : (j['food_id'] as num).toInt(),
        label: j['label'] as String? ?? '',
        quantity: (j['quantity'] as num).toDouble(),
        calories: (j['calories'] as num).toDouble(),
        protein: (j['protein'] as num).toDouble(),
        carbs: (j['carbs'] as num).toDouble(),
        fat: (j['fat'] as num).toDouble(),
        position: (j['position'] as num?)?.toInt() ?? 0,
      );
}

class Meal {
  final int id;
  final String name;
  final String mealType;
  final String portion;
  final String notes;
  final DateTime? eatenAt;
  final DateTime? createdAt;
  final List<MealItem> items;
  final double calories;
  final double protein;
  final double carbs;
  final double fat;

  const Meal({
    required this.id,
    required this.name,
    required this.mealType,
    required this.portion,
    required this.notes,
    required this.items,
    required this.calories,
    required this.protein,
    required this.carbs,
    required this.fat,
    this.eatenAt,
    this.createdAt,
  });

  factory Meal.fromJson(Map<String, dynamic> j) => Meal(
        id: (j['id'] as num).toInt(),
        name: j['name'] as String? ?? '',
        mealType: j['meal_type'] as String? ?? '',
        portion: j['portion'] as String? ?? '',
        notes: j['notes'] as String? ?? '',
        eatenAt: _tryTime(j['eaten_at']),
        createdAt: _tryTime(j['created_at']),
        items: (j['items'] as List<dynamic>? ?? [])
            .map((e) => MealItem.fromJson(e as Map<String, dynamic>))
            .toList(),
        calories: (j['calories'] as num?)?.toDouble() ?? 0,
        protein: (j['protein'] as num?)?.toDouble() ?? 0,
        carbs: (j['carbs'] as num?)?.toDouble() ?? 0,
        fat: (j['fat'] as num?)?.toDouble() ?? 0,
      );
}

class Fast {
  final int id;
  final DateTime? startedAt;
  final DateTime? endedAt;
  final double? targetHours;
  final String notes;
  final DateTime? createdAt;
  final double? durationHours;

  const Fast({
    required this.id,
    required this.notes,
    this.startedAt,
    this.endedAt,
    this.targetHours,
    this.createdAt,
    this.durationHours,
  });

  factory Fast.fromJson(Map<String, dynamic> j) => Fast(
        id: (j['id'] as num).toInt(),
        startedAt: _tryTime(j['started_at']),
        endedAt: _tryTime(j['ended_at']),
        targetHours: (j['target_hours'] as num?)?.toDouble(),
        notes: j['notes'] as String? ?? '',
        createdAt: _tryTime(j['created_at']),
        durationHours: (j['duration_hours'] as num?)?.toDouble(),
      );

  bool get isActive => endedAt == null;
}

class Exercise {
  final int id;
  final String name;
  final String type; // "weighted" | "timed"
  final String muscles;
  final String equipment;
  final bool isSystem;
  final DateTime? createdAt;

  const Exercise({
    required this.id,
    required this.name,
    required this.type,
    required this.muscles,
    required this.equipment,
    required this.isSystem,
    this.createdAt,
  });

  factory Exercise.fromJson(Map<String, dynamic> j) => Exercise(
        id: (j['id'] as num).toInt(),
        name: j['name'] as String? ?? '',
        type: j['type'] as String? ?? 'weighted',
        muscles: j['muscles'] as String? ?? '',
        equipment: j['equipment'] as String? ?? '',
        isSystem: j['is_system'] as bool? ?? false,
        createdAt: _tryTime(j['created_at']),
      );

  bool get isWeighted => type == 'weighted';
}

class WorkoutEntry {
  final int id;
  final int workoutId;
  final int exerciseId;
  final String exerciseName;
  final String exerciseType;
  final double? weightKg;
  final int? reps;
  final double? durationSec;
  final double? distanceKm;
  final int? effort;
  final String notes;
  final int position;

  const WorkoutEntry({
    required this.id,
    required this.workoutId,
    required this.exerciseId,
    required this.exerciseName,
    required this.exerciseType,
    required this.notes,
    required this.position,
    this.weightKg,
    this.reps,
    this.durationSec,
    this.distanceKm,
    this.effort,
  });

  factory WorkoutEntry.fromJson(Map<String, dynamic> j) => WorkoutEntry(
        id: (j['id'] as num).toInt(),
        workoutId: (j['workout_id'] as num).toInt(),
        exerciseId: (j['exercise_id'] as num).toInt(),
        exerciseName: j['exercise_name'] as String? ?? '',
        exerciseType: j['exercise_type'] as String? ?? '',
        weightKg: (j['weight_kg'] as num?)?.toDouble(),
        reps: (j['reps'] as num?)?.toInt(),
        durationSec: (j['duration_sec'] as num?)?.toDouble(),
        distanceKm: (j['distance_km'] as num?)?.toDouble(),
        effort: (j['effort'] as num?)?.toInt(),
        notes: j['notes'] as String? ?? '',
        position: (j['position'] as num?)?.toInt() ?? 0,
      );
}

class Workout {
  final int id;
  final String name;
  final DateTime? startedAt;
  final DateTime? endedAt;
  final int? effort;
  final String notes;
  final DateTime? createdAt;
  final List<WorkoutEntry> entries;
  final double? durationMin;

  const Workout({
    required this.id,
    required this.name,
    required this.notes,
    required this.entries,
    this.startedAt,
    this.endedAt,
    this.effort,
    this.createdAt,
    this.durationMin,
  });

  factory Workout.fromJson(Map<String, dynamic> j) => Workout(
        id: (j['id'] as num).toInt(),
        name: j['name'] as String? ?? '',
        startedAt: _tryTime(j['started_at']),
        endedAt: _tryTime(j['ended_at']),
        effort: (j['effort'] as num?)?.toInt(),
        notes: j['notes'] as String? ?? '',
        createdAt: _tryTime(j['created_at']),
        entries: (j['entries'] as List<dynamic>? ?? [])
            .map((e) => WorkoutEntry.fromJson(e as Map<String, dynamic>))
            .toList(),
        durationMin: (j['duration_min'] as num?)?.toDouble(),
      );

  bool get isFinished => endedAt != null;
}