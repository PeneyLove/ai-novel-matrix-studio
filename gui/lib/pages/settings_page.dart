import 'package:flutter/material.dart';
import '../services/api_client.dart';

class SettingsPage extends StatefulWidget {
  const SettingsPage({super.key});

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage> {
  final _api = ApiClient();
  bool _connected = false;
  bool _checking = false;
  final _portCtrl = TextEditingController(text: '9876');
  final _baseUrlCtrl = TextEditingController(text: 'http://127.0.0.1:9876');

  @override
  void dispose() {
    _portCtrl.dispose();
    _baseUrlCtrl.dispose();
    super.dispose();
  }

  Future<void> _testConnection() async {
    setState(() => _checking = true);
    try {
      await _api.healthCheck();
      setState(() => _connected = true);
    } catch (e) {
      setState(() => _connected = false);
    }
    setState(() => _checking = false);
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Settings', style: Theme.of(context).textTheme.headlineMedium),
          const SizedBox(height: 24),
          Card(
            color: const Color(0xFF181825),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('Harness Connection', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
                  const SizedBox(height: 12),
                  TextField(
                    controller: _baseUrlCtrl,
                    decoration: const InputDecoration(labelText: 'API Base URL', border: OutlineInputBorder()),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      ElevatedButton.icon(
                        onPressed: _checking ? null : _testConnection,
                        icon: _checking
                            ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                            : const Icon(Icons.wifi_find),
                        label: const Text('Test Connection'),
                      ),
                      const SizedBox(width: 16),
                      Icon(
                        _connected ? Icons.check_circle : Icons.error,
                        color: _connected ? const Color(0xFFA6E3A1) : const Color(0xFFF38BA8),
                      ),
                      Text(
                        _connected ? ' Connected' : ' Disconnected',
                        style: TextStyle(color: _connected ? const Color(0xFFA6E3A1) : const Color(0xFFF38BA8)),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 16),
          Card(
            color: const Color(0xFF181825),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('About', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
                  const SizedBox(height: 8),
                  const Text('AI Novel Agent v1.0.0-alpha', style: TextStyle(color: Color(0xFF6C7086))),
                  const Text('Go Harness + Flutter GUI', style: TextStyle(color: Color(0xFF45475A), fontSize: 12)),
                  const Text('All data stored locally in .novelAgent/', style: TextStyle(color: Color(0xFF45475A), fontSize: 12)),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
