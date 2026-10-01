/// Meals tab: meal log with totals, item-level detail, create/edit via an
/// item builder (catalog foods or ad-hoc entries), and the food catalog.
library;

import 'package:flutter/material.dart';

import '../api_client.dart';
import '../app_state.dart';
import '../models.dart';
import '../widgets.dart';

class MealsScreen extends StatefulWidget {
  const MealsScreen({super.key});

  @override
  State<MealsScreen> createState() => _MealsScreenState();
}

class _MealsScreenState extends State<MealsScreen> {
  int _epoch = 0;

  ApiClient get _api => AppStateScope.of(context).client!;

  void _refresh() => setState(() => _epoch++);

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Meals'),
        actions: [
          IconButton(
            icon: const Icon(Icons.restaurant_menu),
            tooltip: 'Food catalog',
            onPressed: () => Navigator.of(context).push(MaterialPageRoute(
                builder: (_) => const FoodCatalogScreen())).then((_) => _refresh()),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _create,
        icon: const Icon(Icons.add),
        label: const Text('Log meal'),
      ),
      body: LoadableList<Meal>(
        key: ValueKey('meals-$_epoch'),
        fetch: _api.listMeals,
        emptyMessage:
            'No meals logged yet.\nTap Log meal to record what you ate.',
        builder: (meals, reload) => ListView.builder(
          padding: const EdgeInsets.only(bottom: 88),
          itemCount: meals.length,
          itemBuilder: (context, i) => _mealTile(context, meals[i]),
        ),
      ),
    );
  }

  Widget _mealTile(BuildContext context, Meal meal) {
    final title = meal.name.isNotEmpty
        ? meal.name
        : (meal.mealType.isNotEmpty ? meal.mealType : 'Meal');
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 4),
        child: Column(
          children: [
            ListTile(
              leading: const Icon(Icons.restaurant),
              title: Text(title),
              subtitle: Text(
                '${fmtDateTime(meal.eatenAt)}'
                '${meal.portion.isNotEmpty ? ' · ${meal.portion}' : ''}'
                '${meal.notes.isNotEmpty ? ' · ${meal.notes}' : ''}',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              trailing: PopupMenuButton<String>(
                onSelected: (action) => action == 'edit'
                    ? _edit(meal)
                    : _delete(meal),
                itemBuilder: (_) => const [
                  PopupMenuItem(value: 'edit', child: Text('Edit')),
                  PopupMenuItem(value: 'delete', child: Text('Delete')),
                ],
              ),
              onTap: () => _showDetail(meal),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
              child: Row(
                children: [
                  _macroBadge(context, fmtValue(meal.calories),
                      'kcal', true),
                  const SizedBox(width: 8),
                  _macroBadge(
                      context, 'P ${fmtValue(meal.protein)}', 'g', false),
                  const SizedBox(width: 8),
                  _macroBadge(
                      context, 'C ${fmtValue(meal.carbs)}', 'g', false),
                  const SizedBox(width: 8),
                  _macroBadge(context, 'F ${fmtValue(meal.fat)}', 'g', false),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _macroBadge(
      BuildContext context, String value, String unit, bool emphasized) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: emphasized
            ? theme.colorScheme.primaryContainer
            : theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        '$value $unit',
        style: theme.textTheme.labelMedium?.copyWith(
          color: emphasized
              ? theme.colorScheme.onPrimaryContainer
              : theme.colorScheme.onSurfaceVariant,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  Future<void> _showDetail(Meal meal) async {
    final fresh = await _api.getMeal(meal.id);
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
          children: [
            Text(
              meal.name.isNotEmpty
                  ? meal.name
                  : (meal.mealType.isNotEmpty ? meal.mealType : 'Meal'),
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 4),
            Text(fmtDateTime(fresh.eatenAt)),
            if (fresh.portion.isNotEmpty)
              Text('Portion: ${fresh.portion}'),
            if (fresh.notes.isNotEmpty) Text('Notes: ${fresh.notes}'),
            const Divider(height: 24),
            ...fresh.items.map((it) => ListTile(
                  contentPadding: EdgeInsets.zero,
                  dense: true,
                  leading: Icon(
                      it.foodId == null ? Icons.edit_note : Icons.lunch_dining),
                  title: Text(it.label),
                  subtitle: it.foodId == null
                      ? null
                      : Text(
                          '${fmtValue(it.quantity)} × serving'),
                  trailing: Text(
                      '${fmtValue(it.calories)} kcal'
                      '${it.protein > 0 || it.carbs > 0 || it.fat > 0 ? '\nP ${fmtValue(it.protein)} C ${fmtValue(it.carbs)} F ${fmtValue(it.fat)}' : ''}',
                      textAlign: TextAlign.right,
                      style: Theme.of(context).textTheme.bodySmall,
                  ),
                )),
          ],
        ),
      ),
    );
  }

  Future<void> _create() async {
    final result = await _mealForm(context, null);
    if (result == null) return;
    try {
      await _api.createMeal(
        name: result.$1,
        mealType: result.$2,
        portion: result.$3,
        notes: result.$4,
        eatenAt: result.$5,
        items: result.$6,
      );
      if (mounted) showSnack(context, 'Meal logged');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }

  Future<void> _edit(Meal meal) async {
    final result = await _mealForm(context, meal);
    if (result == null) return;
    try {
      await _api.updateMeal(
        meal.id,
        name: result.$1,
        mealType: result.$2,
        portion: result.$3,
        notes: result.$4,
        eatenAt: result.$5,
        items: result.$6,
      );
      if (mounted) showSnack(context, 'Meal updated');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }

  Future<void> _delete(Meal meal) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Delete meal?'),
        content: Text('Logged ${fmtDateTime(meal.eatenAt)}.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            style: FilledButton.styleFrom(
                backgroundColor: Theme.of(dialogContext).colorScheme.error),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await _api.deleteMeal(meal.id);
      if (mounted) showSnack(context, 'Deleted');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }
}

/// The meal create/edit form: header fields + a dynamic item list. Items
/// reference catalog foods (quantity in serving units; the server computes
/// nutrition) or carry ad-hoc label + calories/macros.
/// Returns (name, mealType, portion, notes, eatenAt, items).
Future<(String, String, String, String, DateTime?, List<MealItemInput>)?>
    _mealForm(BuildContext context, Meal? existing) async {
  final api = AppStateScope.of(context).client!;
  final foods = await api.listFoods();
  if (!context.mounted) return null;

  final name = TextEditingController(text: existing?.name ?? '');
  final notes = TextEditingController(text: existing?.notes ?? '');
  String mealType = existing?.mealType ?? '';
  String portion = existing?.portion ?? '';
  DateTime? eatenAt = existing?.eatenAt;
  // Seed with the meal's existing items for editing (food_id items keep
  // referencing their food; ad-hoc items keep their numbers).
  final items = <_ItemDraft>[
    ...?existing?.items.map((it) => it.foodId == null
        ? _ItemDraft.adhoc(label: it.label, calories: it.calories,
            protein: it.protein, carbs: it.carbs, fat: it.fat)
        : _ItemDraft.food(foodId: it.foodId!, label: it.label,
            quantity: it.quantity)),
  ];

  final saved = await showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (sheetContext) => StatefulBuilder(
      builder: (sheetContext, setSheetState) {
        final padding = EdgeInsets.only(
          bottom: MediaQuery.of(sheetContext).viewInsets.bottom,
        );
        return Padding(
          padding: padding,
          child: SingleChildScrollView(
            child: SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(existing == null ? 'Log meal' : 'Edit meal',
                        style: Theme.of(context).textTheme.titleLarge),
                    const SizedBox(height: 16),
                    TextField(
                      controller: name,
                      decoration: const InputDecoration(
                          labelText: 'Name (optional)',
                          border: OutlineInputBorder()),
                    ),
                    const SizedBox(height: 12),
                    DropdownButtonFormField<String>(
                      initialValue: mealType,
                      decoration: const InputDecoration(
                          labelText: 'Meal type', border: OutlineInputBorder()),
                      items: const [
                        DropdownMenuItem(value: '', child: Text('—')),
                        DropdownMenuItem(value: 'breakfast', child: Text('Breakfast')),
                        DropdownMenuItem(value: 'lunch', child: Text('Lunch')),
                        DropdownMenuItem(value: 'dinner', child: Text('Dinner')),
                        DropdownMenuItem(value: 'snack', child: Text('Snack')),
                      ],
                      onChanged: (v) => setSheetState(() => mealType = v ?? ''),
                    ),
                    const SizedBox(height: 12),
                    DropdownButtonFormField<String>(
                      initialValue: portion,
                      decoration: const InputDecoration(
                          labelText: 'Portion (optional)',
                          border: OutlineInputBorder()),
                      items: const [
                        DropdownMenuItem(value: '', child: Text('—')),
                        DropdownMenuItem(value: 'small', child: Text('Small')),
                        DropdownMenuItem(value: 'medium', child: Text('Medium')),
                        DropdownMenuItem(value: 'large', child: Text('Large')),
                      ],
                      onChanged: (v) => setSheetState(() => portion = v ?? ''),
                    ),
                    const SizedBox(height: 12),
                    OutlinedButton.icon(
                      onPressed: () async {
                        final picked = await showDatePicker(
                          context: sheetContext,
                          initialDate: eatenAt ?? DateTime.now(),
                          firstDate: DateTime(2000),
                          lastDate: DateTime.now(),
                        );
                        if (picked != null) {
                          setSheetState(() => eatenAt = picked);
                        }
                      },
                      icon: const Icon(Icons.calendar_today, size: 16),
                      label: Text(eatenAt == null
                          ? 'Eaten now'
                          : 'Eaten ${fmtDate(eatenAt)}'),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: notes,
                      decoration: const InputDecoration(
                          labelText: 'Notes (optional)',
                          border: OutlineInputBorder()),
                    ),
                    const SizedBox(height: 16),
                    Row(
                      children: [
                        Text('Items',
                            style: Theme.of(context).textTheme.titleMedium),
                        const Spacer(),
                        IconButton(
                          tooltip: 'Add food from catalog',
                          onPressed: () async {
                            final draft = await _pickFood(sheetContext, foods);
                            if (draft != null) {
                              setSheetState(() => items.add(draft));
                            }
                          },
                          icon: const Icon(Icons.restaurant_menu),
                        ),
                        IconButton(
                          tooltip: 'Add custom item',
                          onPressed: () async {
                            final draft = await _adhocItemSheet(sheetContext);
                            if (draft != null) {
                              setSheetState(() => items.add(draft));
                            }
                          },
                          icon: const Icon(Icons.edit_note),
                        ),
                      ],
                    ),
                    for (final (i, item) in items.indexed)
                      ListTile(
                        dense: true,
                        contentPadding: EdgeInsets.zero,
                        leading: Icon(item.foodId == null
                            ? Icons.edit_note
                            : Icons.lunch_dining),
                        title: Text(item.label),
                        subtitle: item.foodId == null
                            ? Text('${fmtValue(item.calories)} kcal')
                            : Text(
                                '${fmtValue(item.quantity)} × serving'),
                        trailing: IconButton(
                          icon: const Icon(Icons.remove_circle_outline),
                          onPressed: () =>
                              setSheetState(() => items.removeAt(i)),
                        ),
                      ),
                    if (items.isEmpty)
                      const Padding(
                        padding: EdgeInsets.symmetric(vertical: 8),
                        child: Text(
                          'No items yet — add a catalog food or a custom entry.',
                          style: TextStyle(fontStyle: FontStyle.italic),
                        ),
                      ),
                    const SizedBox(height: 16),
                    FilledButton(
                      onPressed: () => Navigator.pop(sheetContext, true),
                      child: Text(existing == null ? 'Save meal' : 'Update meal'),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    ),
  );
  if (saved != true) return null;
  return (
    name.text.trim(),
    mealType,
    portion,
    notes.text.trim(),
    eatenAt,
    items
        .map((d) => d.foodId != null
            ? MealItemInput(
                foodId: d.foodId,
                quantity: d.quantity,
                label: d.label,
              )
            : MealItemInput(
                label: d.label,
                calories: d.calories,
                protein: d.protein,
                carbs: d.carbs,
                fat: d.fat,
              ))
        .toList(),
  );
}

/// One item being drafted in the meal form.
class _ItemDraft {
  final int? foodId;
  final String label;
  final double quantity;
  final double calories;
  final double protein;
  final double carbs;
  final double fat;

  const _ItemDraft({
    this.foodId,
    this.label = '',
    this.quantity = 1,
    this.calories = 0,
    this.protein = 0,
    this.carbs = 0,
    this.fat = 0,
  });

  factory _ItemDraft.food({
    required int foodId,
    required String label,
    required double quantity,
  }) =>
      _ItemDraft(
          foodId: foodId, label: label, quantity: quantity);

  factory _ItemDraft.adhoc({
    required String label,
    required double calories,
    double protein = 0,
    double carbs = 0,
    double fat = 0,
  }) =>
      _ItemDraft(
          label: label,
          calories: calories,
          protein: protein,
          carbs: carbs,
          fat: fat);
}

/// Catalog food picker with serving quantity; returns a food item draft.
Future<_ItemDraft?> _pickFood(BuildContext context, List<Food> foods) {
  final queryCtrl = TextEditingController();
  return showModalBottomSheet<_ItemDraft>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (sheetContext) => StatefulBuilder(
      builder: (sheetContext, setSheetState) {
        var query = '';
        return Padding(
          padding: EdgeInsets.only(
              bottom: MediaQuery.of(sheetContext).viewInsets.bottom),
          child: SafeArea(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 20),
                  child: TextField(
                    controller: queryCtrl,
                    decoration: const InputDecoration(
                        labelText: 'Search foods',
                        prefixIcon: Icon(Icons.search),
                        border: OutlineInputBorder()),
                    onChanged: (v) => setSheetState(() => query = v),
                  ),
                ),
                Flexible(
                  child: ListView(
                    shrinkWrap: true,
                    children: foods
                        .where((f) =>
                            query.isEmpty ||
                            f.name.toLowerCase().contains(query) ||
                            f.brand.toLowerCase().contains(query))
                        .map((f) => ListTile(
                              title: Text(f.name),
                              subtitle: Text(
                                  '${f.servingLabel} · ${fmtValue(f.calories)} kcal'),
                              trailing: const Icon(Icons.add),
                              onTap: () async {
                                final qty = await _quantitySheet(
                                    sheetContext, f);
                                if (qty == null || !sheetContext.mounted) {
                                  return;
                                }
                                Navigator.pop(
                                    sheetContext,
                                    _ItemDraft.food(
                                      foodId: f.id,
                                      label: f.name,
                                      quantity: qty,
                                    ));
                              },
                            ))
                        .toList(),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    ),
  );
}

/// Quantity entry for a picked food. The API scales nutrition as
/// quantity / serving_size — i.e. quantity is measured in the food's
/// serving unit (grams, ml, eggs, slices, ...).
Future<double?> _quantitySheet(BuildContext context, Food food) {
  final ctrl = TextEditingController(text: '${food.servingSize}');
  return showDialog<double>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text(food.name),
      content: TextField(
        controller: ctrl,
        decoration: InputDecoration(
          labelText: 'Amount (${food.servingUnit}; ${fmtValue(food.calories)} kcal per ${food.servingLabel})',
          border: const OutlineInputBorder(),
        ),
        keyboardType: const TextInputType.numberWithOptions(decimal: true),
        autofocus: true,
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('Cancel')),
        FilledButton(
          onPressed: () {
            final q = double.tryParse(ctrl.text);
            if (q == null || q <= 0) return;
            Navigator.pop(dialogContext, q);
          },
          child: const Text('Add'),
        ),
      ],
    ),
  );
}

/// Ad-hoc item entry: label + calories (+ optional macros).
Future<_ItemDraft?> _adhocItemSheet(BuildContext context) {
  final labelCtrl = TextEditingController();
  final kcalCtrl = TextEditingController();
  final proteinCtrl = TextEditingController();
  final carbsCtrl = TextEditingController();
  final fatCtrl = TextEditingController();
  return showModalBottomSheet<_ItemDraft>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (sheetContext) => Padding(
      padding: EdgeInsets.only(
          bottom: MediaQuery.of(sheetContext).viewInsets.bottom),
      child: SingleChildScrollView(
        child: SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text('Custom item',
                    style: Theme.of(context).textTheme.titleLarge),
                const SizedBox(height: 16),
                TextField(
                  controller: labelCtrl,
                  decoration: const InputDecoration(
                      labelText: 'Label', border: OutlineInputBorder()),
                  autofocus: true,
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: kcalCtrl,
                  decoration: const InputDecoration(
                      labelText: 'Calories', border: OutlineInputBorder()),
                  keyboardType:
                      const TextInputType.numberWithOptions(decimal: true),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                        child: TextField(
                      controller: proteinCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Protein g',
                          border: OutlineInputBorder()),
                      keyboardType: const TextInputType.numberWithOptions(
                          decimal: true),
                    )),
                    const SizedBox(width: 8),
                    Expanded(
                        child: TextField(
                      controller: carbsCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Carbs g',
                          border: OutlineInputBorder()),
                      keyboardType: const TextInputType.numberWithOptions(
                          decimal: true),
                    )),
                    const SizedBox(width: 8),
                    Expanded(
                        child: TextField(
                      controller: fatCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Fat g', border: OutlineInputBorder()),
                      keyboardType: const TextInputType.numberWithOptions(
                          decimal: true),
                    )),
                  ],
                ),
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: () {
                    final kcal = double.tryParse(kcalCtrl.text) ?? 0;
                    final label = labelCtrl.text.trim();
                    final p = double.tryParse(proteinCtrl.text) ?? 0;
                    final c = double.tryParse(carbsCtrl.text) ?? 0;
                    final f = double.tryParse(fatCtrl.text) ?? 0;
                    if (label.isEmpty) {
                      ScaffoldMessenger.of(sheetContext).showSnackBar(
                          const SnackBar(
                              content: Text('Label is required')));
                      return;
                    }
                    if (kcal <= 0 && p <= 0 && c <= 0 && f <= 0) {
                      ScaffoldMessenger.of(sheetContext).showSnackBar(
                          const SnackBar(
                              content: Text(
                                  'Enter calories or at least one macro')));
                      return;
                    }
                    Navigator.pop(
                        sheetContext,
                        _ItemDraft.adhoc(
                            label: label,
                            calories: kcal,
                            protein: p,
                            carbs: c,
                            fat: f));
                  },
                  child: const Text('Add item'),
                ),
              ],
            ),
          ),
        ),
      ),
    ),
  );
}

/// Food catalog: list all, create/edit/delete custom foods.
class FoodCatalogScreen extends StatefulWidget {
  const FoodCatalogScreen({super.key});

  @override
  State<FoodCatalogScreen> createState() => _FoodCatalogScreenState();
}

class _FoodCatalogScreenState extends State<FoodCatalogScreen> {
  int _epoch = 0;

  void _refresh() => setState(() => _epoch++);

  @override
  Widget build(BuildContext context) {
    final api = AppStateScope.of(context).client!;
    return Scaffold(
      appBar: AppBar(title: const Text('Foods')),
      floatingActionButton: FloatingActionButton(
        tooltip: 'New food',
        onPressed: () async {
          final result = await _foodForm(context, null);
          if (result == null) return;
          try {
            await api.createFood(
                name: result.$1,
                brand: result.$2,
                servingSize: result.$3,
                servingUnit: result.$4,
                calories: result.$5,
                protein: result.$6,
                carbs: result.$7,
                fat: result.$8);
            if (context.mounted) showSnack(context, 'Food created');
          } on ApiException catch (e) {
            if (context.mounted) showSnack(context, e.message, error: true);
          }
          _refresh();
        },
        child: const Icon(Icons.add),
      ),
      body: LoadableList<Food>(
        key: ValueKey('foods-$_epoch'),
        fetch: api.listFoods,
        emptyMessage: 'No foods in the catalog.',
        builder: (items, _) => ListView.builder(
          padding: const EdgeInsets.only(bottom: 88),
          itemCount: items.length,
          itemBuilder: (context, i) {
            final f = items[i];
            return ListTile(
              leading: const Icon(Icons.lunch_dining),
              title: Text(f.brand.isEmpty ? f.name : '${f.name} (${f.brand})'),
              subtitle: Text(
                  '${f.servingLabel} · ${fmtValue(f.calories)} kcal · '
                  'P ${fmtValue(f.protein)} C ${fmtValue(f.carbs)} F ${fmtValue(f.fat)}'),
              trailing: f.isSystem
                  ? const Tooltip(
                      message: 'System food — immutable',
                      child: Icon(Icons.lock_outline))
                  : PopupMenuButton<String>(
                      onSelected: (action) async {
                        if (action == 'edit') {
                          final result = await _foodForm(context, f);
                          if (result == null) return;
                          try {
                            await api.updateFood(f.id,
                                name: result.$1,
                                brand: result.$2,
                                servingSize: result.$3,
                                servingUnit: result.$4,
                                calories: result.$5,
                                protein: result.$6,
                                carbs: result.$7,
                                fat: result.$8);
                            if (context.mounted) {
                              showSnack(context, 'Food updated');
                            }
                          } on ApiException catch (e) {
                            if (context.mounted) {
                              showSnack(context, e.message, error: true);
                            }
                          }
                        } else {
                          final ok = await showDialog<bool>(
                            context: context,
                            builder: (d) => AlertDialog(
                              title: const Text('Delete food?'),
                              content: Text(
                                  '${f.name} will be removed from the catalog.'),
                              actions: [
                                TextButton(
                                    onPressed: () =>
                                        Navigator.pop(d, false),
                                    child: const Text('Cancel')),
                                FilledButton(
                                  onPressed: () => Navigator.pop(d, true),
                                  style: FilledButton.styleFrom(
                                      backgroundColor:
                                          Theme.of(d).colorScheme.error),
                                  child: const Text('Delete'),
                                ),
                              ],
                            ),
                          );
                          if (ok != true) return;
                          try {
                            await api.deleteFood(f.id);
                            if (context.mounted) showSnack(context, 'Deleted');
                          } on ApiException catch (e) {
                            if (context.mounted) {
                              showSnack(context, e.message, error: true);
                            }
                          }
                        }
                        _refresh();
                      },
                      itemBuilder: (_) => const [
                        PopupMenuItem(value: 'edit', child: Text('Edit')),
                        PopupMenuItem(value: 'delete', child: Text('Delete')),
                      ],
                    ),
            );
          },
        ),
      ),
    );
  }
}

/// Create/edit form; returns (name, brand, size, unit, kcal, P, C, F).
Future<(String, String, double, String, double, double, double, double)?>
    _foodForm(BuildContext context, Food? existing) {
  final name =
      TextEditingController(text: existing?.name ?? '');
  final brand = TextEditingController(text: existing?.brand ?? '');
  final size = TextEditingController(
      text: existing == null ? '100' : fmtValue(existing.servingSize));
  final unit = TextEditingController(text: existing?.servingUnit ?? 'g');
  final kcal = TextEditingController(
      text: existing == null ? '' : fmtValue(existing.calories));
  final p = TextEditingController(
      text: existing == null ? '' : fmtValue(existing.protein));
  final c = TextEditingController(
      text: existing == null ? '' : fmtValue(existing.carbs));
  final f = TextEditingController(
      text: existing == null ? '' : fmtValue(existing.fat));
  return showDialog<
      (String, String, double, String, double, double, double, double)>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text(existing == null ? 'New food' : 'Edit food'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
                controller: name,
                decoration: const InputDecoration(
                    labelText: 'Name', border: OutlineInputBorder()),
                autofocus: true),
            const SizedBox(height: 12),
            TextField(
                controller: brand,
                decoration: const InputDecoration(
                    labelText: 'Brand (optional)', border: OutlineInputBorder())),
            const SizedBox(height: 12),
            Row(children: [
              Expanded(
                  child: TextField(
                controller: size,
                decoration: const InputDecoration(
                    labelText: 'Serving size', border: OutlineInputBorder()),
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true),
              )),
              const SizedBox(width: 8),
              Expanded(
                  child: TextField(
                controller: unit,
                decoration: const InputDecoration(
                    labelText: 'Unit', border: OutlineInputBorder()),
              )),
            ]),
            const SizedBox(height: 12),
            TextField(
                controller: kcal,
                decoration: const InputDecoration(
                    labelText: 'Calories', border: OutlineInputBorder()),
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true)),
            const SizedBox(height: 12),
            Row(children: [
              Expanded(
                  child: TextField(
                controller: p,
                decoration: const InputDecoration(
                    labelText: 'Protein g', border: OutlineInputBorder()),
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true),
              )),
              const SizedBox(width: 8),
              Expanded(
                  child: TextField(
                controller: c,
                decoration: const InputDecoration(
                    labelText: 'Carbs g', border: OutlineInputBorder()),
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true),
              )),
              const SizedBox(width: 8),
              Expanded(
                  child: TextField(
                controller: f,
                decoration: const InputDecoration(
                    labelText: 'Fat g', border: OutlineInputBorder()),
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true),
              )),
            ]),
          ],
        ),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('Cancel')),
        FilledButton(
          onPressed: () {
            final parsed = (double.tryParse(size.text) ?? 0) <= 0 ||
                (double.tryParse(kcal.text) ?? 0) < 0;
            if (name.text.trim().isEmpty || parsed) {
              ScaffoldMessenger.of(dialogContext).showSnackBar(const SnackBar(
                  content: Text('Name and a positive serving size are required')));
              return;
            }
            Navigator.pop(dialogContext, (
              name.text.trim(),
              brand.text.trim(),
              double.tryParse(size.text) ?? 0,
              unit.text.trim().isEmpty ? 'g' : unit.text.trim(),
              double.tryParse(kcal.text) ?? 0,
              double.tryParse(p.text) ?? 0,
              double.tryParse(c.text) ?? 0,
              double.tryParse(f.text) ?? 0,
            ));
          },
          child: const Text('Save'),
        ),
      ],
    ),
  );
}