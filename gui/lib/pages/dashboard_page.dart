import 'package:flutter/material.dart';
import '../services/api_client.dart';

class DashboardPage extends StatefulWidget {
  const DashboardPage({super.key});

  @override
  State<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends State<DashboardPage> {
  final _api = ApiClient();
  bool _loading = true;
  String _status = '';
  int _taskCount = 0;
  int _skillCount = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final health = await _api.healthCheck();
      final tasks = await _api.listTasks();
      final skills = await _api.listSkills();
      setState(() {
        _status = health['status'] as String? ?? 'unknown';
        _taskCount = tasks.length;
        _skillCount = skills.length;
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _status = 'disconnected: $e';
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Dashboard', style: Theme.of(context).textTheme.headlineMedium),
          const SizedBox(height: 24),
          if (_loading)
            const CircularProgressIndicator()
          else ...[
            Row(
              children: [
                _StatCard(label: 'Harness', value: _status, color: const Color(0xFFA6E3A1)),
                const SizedBox(width: 16),
                _StatCard(label: 'Tasks', value: '$_taskCount', color: const Color(0xFF89B4FA)),
                const SizedBox(width: 16),
                _StatCard(label: 'Skills', value: '$_skillCount', color: const Color(0xFFCBA6F7)),
              ],
            ),
            const SizedBox(height: 24),
            ElevatedButton.icon(
              onPressed: _load,
              icon: const Icon(Icons.refresh),
              label: const Text('Refresh'),
            ),
          ],
        ],
      ),
    );
  }
}

class _StatCard extends StatelessWidget {
  final String label;
  final String value;
  final Color color;

  const _StatCard({required this.label, required this.value, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 150,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF181825),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFF313244)),
      ),
      child: Column(
        children: [
          Text(value, style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold, color: color)),
          const SizedBox(height: 4),
          Text(label, style: const TextStyle(color: Color(0xFF6C7086))),
        ],
      ),
    );
  }
}
